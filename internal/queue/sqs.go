package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/dev-khalid/hookwave/internal/events"
)

// Config holds the parameters needed to connect to an SQS-compatible endpoint.
type Config struct {
	QueueName          string
	Endpoint           string // empty = real AWS; non-empty = override (ElasticMQ, etc.)
	Region             string
	AWSAccessKeyID     string // empty = use SDK default credential chain
	AWSSecretAccessKey string
}

// Publisher is satisfied by Client and useful for test doubles.
type Publisher interface {
	Publish(ctx context.Context, body []byte, attrs map[string]string) error
}

// Consumer is satisfied by Client and useful for test doubles.
type Consumer interface {
	ReceiveMessages(ctx context.Context, maxCount int32) ([]Message, error)
	DeleteMessage(ctx context.Context, receiptHandle string) error
	ChangeMessageVisibility(ctx context.Context, receiptHandle string, seconds int32) error
}

// Message is a clean wrapper around an SQS message.
type Message struct {
	MessageID     string
	ReceiptHandle string
	Body          []byte
	Attributes    map[string]string
}

// Client wraps the AWS SQS SDK and satisfies both Publisher and Consumer.
type Client struct {
	awsSqsClient *sqs.Client
	queueURL     string
}

const DefaultWaitTimeSeconds int32 = 20
const DefaultMessageVisibilityTimeout = "60"
const DefaultMaxReceiveCount = 3
const dlqSuffix = "-dlq"

// NewClient creates a Client and ensures the queue exists.
//
// Credential resolution:
//   - cfg.AWSAccessKeyID non-empty → static credentials (local / CI)
//   - cfg.AWSAccessKeyID empty     → SDK default chain (env → ~/.aws → IMDS → ECS/EKS role)
//
// Endpoint resolution:
//   - cfg.Endpoint non-empty → override (ElasticMQ or any custom endpoint)
//   - cfg.Endpoint empty     → real AWS endpoint
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	if cfg.AWSAccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	awsSqsClient := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	c := &Client{awsSqsClient: awsSqsClient}

	dlqArn, err := c.ensureDLQ(ctx, cfg.QueueName+dlqSuffix)
	if err != nil {
		return nil, err
	}

	if err := c.ensureQueue(ctx, cfg.QueueName, dlqArn); err != nil {
		return nil, err
	}

	return c, nil
}

// QueueURL returns the resolved queue URL.
func (c *Client) QueueURL() string { return c.queueURL }

// PublishEvent marshals a typed event to JSON and publishes it.
// Only concrete types listed in ListedEventTypes are accepted — others are a compile error.
func PublishEvent[E events.ListedEventTypes](ctx context.Context, c Publisher, event E, attrs map[string]string) error {
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return c.Publish(ctx, b, attrs)
}

// Publish sends a raw message body. Satisfies the Publisher interface.
func (c *Client) Publish(ctx context.Context, body []byte, attrs map[string]string) error {
	_, err := c.awsSqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(c.queueURL),
		MessageBody:       aws.String(string(body)),
		MessageAttributes: StringAttributes(attrs),
	})
	if err != nil {
		return fmt.Errorf("SQS SendMessage: %w", err)
	}
	return nil
}

// SendMessage sends a message and returns the SQS-assigned message ID.
func (c *Client) SendMessage(ctx context.Context, body []byte, attrs map[string]string) (string, error) {
	out, err := c.awsSqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(c.queueURL),
		MessageBody:       aws.String(string(body)),
		MessageAttributes: StringAttributes(attrs),
	})
	if err != nil {
		return "", fmt.Errorf("SQS SendMessage: %w", err)
	}
	return aws.ToString(out.MessageId), nil
}

// ReceiveMessages long-polls the queue.
func (c *Client) ReceiveMessages(ctx context.Context, maxCount int32) ([]Message, error) {
	out, err := c.awsSqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(c.queueURL),
		MaxNumberOfMessages:   maxCount,
		WaitTimeSeconds:       DefaultWaitTimeSeconds,
		MessageAttributeNames: []string{"All"},
	})
	if err != nil {
		return nil, fmt.Errorf("SQS ReceiveMessage: %w", err)
	}

	msgs := make([]Message, 0, len(out.Messages))
	for _, m := range out.Messages {
		msgs = append(msgs, toMessage(m))
	}
	return msgs, nil
}

// DeleteMessage acknowledges a message, removing it from the queue.
func (c *Client) DeleteMessage(ctx context.Context, receiptHandle string) error {
	_, err := c.awsSqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("SQS DeleteMessage: %w", err)
	}
	return nil
}

// ChangeMessageVisibility extends or reduces the visibility timeout of an in-flight message.
func (c *Client) ChangeMessageVisibility(ctx context.Context, receiptHandle string, seconds int32) error {
	_, err := c.awsSqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(c.queueURL),
		ReceiptHandle:     aws.String(receiptHandle),
		VisibilityTimeout: seconds,
	})
	if err != nil {
		return fmt.Errorf("SQS ChangeMessageVisibility: %w", err)
	}
	return nil
}

// ensureQueue gets-or-creates the main queue and (re)applies its attributes,
// including the redrive policy — this keeps existing queues (created before
// the DLQ was wired up) in sync, not just newly-created ones.
func (c *Client) ensureQueue(ctx context.Context, name string, dlqArn string) error {
	attrs := map[string]string{
		"ReceiveMessageWaitTimeSeconds": fmt.Sprintf("%d", DefaultWaitTimeSeconds),
		"VisibilityTimeout":             DefaultMessageVisibilityTimeout,
		"RedrivePolicy":                 fmt.Sprintf(`{"deadLetterTargetArn":%q,"maxReceiveCount":"%d"}`, dlqArn, DefaultMaxReceiveCount),
	}

	out, err := c.awsSqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	if err == nil && aws.ToString(out.QueueUrl) != "" {
		c.queueURL = *out.QueueUrl
		if _, setErr := c.awsSqsClient.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
			QueueUrl:   aws.String(c.queueURL),
			Attributes: attrs,
		}); setErr != nil {
			return fmt.Errorf("set attributes on SQS queue %q: %w", name, setErr)
		}
		return nil
	}

	created, createErr := c.awsSqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name), Attributes: attrs})
	if createErr != nil {
		return fmt.Errorf("ensure SQS queue %q (get: %v): %w", name, err, createErr)
	}
	if aws.ToString(created.QueueUrl) == "" {
		return fmt.Errorf("create SQS queue %q returned empty URL", name)
	}

	c.queueURL = *created.QueueUrl
	return nil
}

// ensureDLQ gets-or-creates the dead-letter queue for name and returns its ARN,
// which ensureQueue needs to wire the main queue's RedrivePolicy.
func (c *Client) ensureDLQ(ctx context.Context, name string) (string, error) {
	out, err := c.awsSqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	var dlqURL string
	if err == nil {
		dlqURL = aws.ToString(out.QueueUrl)
	}
	if dlqURL == "" {
		created, createErr := c.awsSqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name)})
		if createErr != nil {
			return "", fmt.Errorf("ensure DLQ %q (get: %v): %w", name, err, createErr)
		}
		dlqURL = aws.ToString(created.QueueUrl)
		if dlqURL == "" {
			return "", fmt.Errorf("create DLQ %q returned empty URL", name)
		}
	}

	attrsOut, err := c.awsSqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(dlqURL),
		AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameQueueArn},
	})
	if err != nil {
		return "", fmt.Errorf("get DLQ %q ARN: %w", name, err)
	}
	arn := attrsOut.Attributes[string(types.QueueAttributeNameQueueArn)]
	if arn == "" {
		return "", fmt.Errorf("DLQ %q has no ARN", name)
	}

	return arn, nil
}

func toMessage(m types.Message) Message {
	msg := Message{
		MessageID:     aws.ToString(m.MessageId),
		ReceiptHandle: aws.ToString(m.ReceiptHandle),
		Body:          []byte(aws.ToString(m.Body)),
		Attributes:    make(map[string]string, len(m.MessageAttributes)),
	}
	for k, v := range m.MessageAttributes {
		if v.StringValue != nil {
			msg.Attributes[k] = *v.StringValue
		}
	}
	return msg
}

// StringAttributes converts a plain map to SQS MessageAttributeValue map.
func StringAttributes(attrs map[string]string) map[string]types.MessageAttributeValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]types.MessageAttributeValue, len(attrs))
	for k, v := range attrs {
		out[k] = types.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(v),
		}
	}
	return out
}

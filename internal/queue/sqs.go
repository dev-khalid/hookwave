package queue

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/dev-khalid/hookwave/internal/config"
)

// Publisher is satisfied by Client and useful for test doubles.
type Publisher interface {
	Publish(ctx context.Context, body []byte, attrs map[string]string) error
}

// Consumer is satisfied by Client and useful for test doubles.
type Consumer interface {
	ReceiveMessages(ctx context.Context, maxCount int32, waitSeconds int32) ([]Message, error)
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
	inner    *sqs.Client
	queueURL string
}

// NewClient creates a Client and ensures the queue exists.
//
// Credential resolution:
//   - cfg.AWSAccessKeyID non-empty → static credentials (local / CI)
//   - cfg.AWSAccessKeyID empty     → SDK default chain (env → ~/.aws → IMDS → ECS/EKS role)
//
// Endpoint resolution:
//   - cfg.Endpoint non-empty → override (ElasticMQ or any custom endpoint)
//   - cfg.Endpoint empty     → real AWS endpoint
func NewClient(ctx context.Context, cfg config.SQSConfig) (*Client, error) {
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

	inner := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	c := &Client{inner: inner}
	if err := c.ensureQueue(ctx, cfg.QueueName); err != nil {
		return nil, err
	}

	return c, nil
}

// QueueURL returns the resolved queue URL.
func (c *Client) QueueURL() string { return c.queueURL }

// Publish sends a message. Satisfies the Publisher interface.
func (c *Client) Publish(ctx context.Context, body []byte, attrs map[string]string) error {
	_, err := c.inner.SendMessage(ctx, &sqs.SendMessageInput{
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
	out, err := c.inner.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(c.queueURL),
		MessageBody:       aws.String(string(body)),
		MessageAttributes: StringAttributes(attrs),
	})
	if err != nil {
		return "", fmt.Errorf("SQS SendMessage: %w", err)
	}
	return aws.ToString(out.MessageId), nil
}

// ReceiveMessages long-polls the queue. Use waitSeconds=20 for max long-poll efficiency.
func (c *Client) ReceiveMessages(ctx context.Context, maxCount int32, waitSeconds int32) ([]Message, error) {
	out, err := c.inner.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(c.queueURL),
		MaxNumberOfMessages:   maxCount,
		WaitTimeSeconds:       waitSeconds,
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
	_, err := c.inner.DeleteMessage(ctx, &sqs.DeleteMessageInput{
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
	_, err := c.inner.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(c.queueURL),
		ReceiptHandle:     aws.String(receiptHandle),
		VisibilityTimeout: seconds,
	})
	if err != nil {
		return fmt.Errorf("SQS ChangeMessageVisibility: %w", err)
	}
	return nil
}

func (c *Client) ensureQueue(ctx context.Context, name string) error {
	out, err := c.inner.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	if err == nil && aws.ToString(out.QueueUrl) != "" {
		c.queueURL = *out.QueueUrl
		return nil
	}

	created, createErr := c.inner.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name)})
	if createErr != nil {
		return fmt.Errorf("ensure SQS queue %q (get: %v): %w", name, err, createErr)
	}
	if aws.ToString(created.QueueUrl) == "" {
		return fmt.Errorf("create SQS queue %q returned empty URL", name)
	}

	c.queueURL = *created.QueueUrl
	return nil
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

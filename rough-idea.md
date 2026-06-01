# Context
I would like to create a webhook-processor app and a fake event-producer app using golang.
And there will be a app to which I will send the webhook data from the webhook-processor. 
I want all them using golang at scale for sure. 
They must be containerized from the beginning. 
In local they should be managed docker-compse.yaml. 

Let's go one by one, and let's plan with you. 

## Services needed
1. Webhook fake event producer -> sent to AWS SQS. (For local, I would like to use this, https://github.com/softwaremill/elasticmq ). 
2. Webhook-processor -> consumes message from sqs queues and send to upstream serivces.
3. The upstream service that webhook-processor will hit and send the payload, this service will store all the data related to webhook inside s3 (MinIO for local)

## Extra functionality 
1. I should be able to monitor each part of the application using opentlemetry, (suggest me open-source for this, that I can host in local)
2. Graphana and promethues for monitoring where is the bottleneck, is it data layer or the application layer or what else. 
3. All the microservices should have the ability to scale to million level once we increase the load and resources. 
4. In local I would like to use kubernetes,helm so that I can install keda based on the queue pressure of sqs. 

## Final thoughs and todo
I don't have the name of the project for now in my mind. Could you suggest a good name fo this? And also share your architectural and tools, packages planning with me in .md file including proper mermaid charts. I would like to use some of the existing golang folder structures/patterns that is available in github/gitlab. I would also like to manage the services in mono-repo style, so that shared items goes to single place and managing codebase becomes simpler.

## Few things you have to keep in mind
I am a begineer in golang. Please list down topics that I should study before starting implementation. Also break down the sprint planning including estimated hours, sequence of development etc. Don't add extra things as mendate, you can add them in nice-to-have section.
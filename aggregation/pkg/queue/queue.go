package queue

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/georgemblack/blue-report/pkg/config"
	"github.com/georgemblack/blue-report/pkg/util"
)

type Queue struct {
	client   *sqs.Client
	queueURL string
}

type Message struct {
	URL string `json:"url"`
}

func New(cfg config.Config) (Queue, error) {
	config, err := awsConfig.LoadDefaultConfig(context.Background(), awsConfig.WithRegion("us-west-2"))
	if err != nil {
		return Queue{}, util.WrapErr("failed to load aws config", err)
	}
	client := sqs.NewFromConfig(config)

	// Use the client to fetch the queue URL
	queueURL, err := client.GetQueueUrl(context.Background(), &sqs.GetQueueUrlInput{
		QueueName: aws.String(cfg.NoralizationQueueName),
	})
	if err != nil {
		return Queue{}, util.WrapErr("failed to get queue url", err)
	}

	return Queue{
		client:   client,
		queueURL: *queueURL.QueueUrl,
	}, nil
}

func (q Queue) Send(msg Message) error {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return util.WrapErr("failed to marshal message", err)
	}

	_, err = q.client.SendMessage(context.Background(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.queueURL),
		MessageBody: aws.String(string(bytes)),
	})
	if err != nil {
		return util.WrapErr("failed to send message", err)
	}

	return nil
}

func (q Queue) Receive() ([]Message, error) {
	messages, err := q.client.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     10,
	})
	if err != nil {
		return []Message{}, util.WrapErr("failed to receive messages", err)
	}

	for _, message := range messages.Messages {
		_, err := q.client.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: message.ReceiptHandle,
		})
		if err != nil {
			return []Message{}, util.WrapErr("failed to delete message", err)
		}
	}

	result := make([]Message, 0, len(messages.Messages))
	for _, message := range messages.Messages {
		msg := Message{}
		err := json.Unmarshal([]byte(*message.Body), &msg)
		if err != nil {
			return []Message{}, util.WrapErr("failed to unmarshal message", err)
		}
		result = append(result, msg)
	}

	return result, nil
}

package email

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type Message struct {
	Subject  string
	BodyText string
	BodyHTML string
	Charset  string
}

type SESService struct {
	client *sesv2.Client
}

func NewSESService() (*SESService, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sesv2.NewFromConfig(cfg)

	return &SESService{
		client: client,
	}, nil
}

func NewMessage() *Message {
	return &Message{
		Charset: "UTF-8",
	}
}

func (s *SESService) SendEmail(from string, to string, message *Message) error {
	return s.SendEmailToMultiple(from, []string{to}, message)
}

func (s *SESService) SendEmailToMultiple(from string, toAddresses []string, message *Message) error {
	// Prepare destinations
	destinations := make([]string, len(toAddresses))
	for i, addr := range toAddresses {
		destinations[i] = addr
	}

	// Prepare email content
	content := &types.EmailContent{
		Simple: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(message.Subject),
				Charset: aws.String(message.Charset),
			},
		},
	}

	// Add body content
	if message.BodyText != "" || message.BodyHTML != "" {
		body := &types.Body{}
		
		if message.BodyText != "" {
			body.Text = &types.Content{
				Data:    aws.String(message.BodyText),
				Charset: aws.String(message.Charset),
			}
		}
		
		if message.BodyHTML != "" {
			body.Html = &types.Content{
				Data:    aws.String(message.BodyHTML),
				Charset: aws.String(message.Charset),
			}
		}
		
		content.Simple.Body = body
	}

	// Send email
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination: &types.Destination{
			ToAddresses: destinations,
		},
		Content: content,
	}

	_, err := s.client.SendEmail(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *SESService) VerifyEmailAddress(email string) error {
	input := &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(email),
	}

	_, err := s.client.CreateEmailIdentity(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to verify email address: %w", err)
	}

	return nil
}

func (s *SESService) ListVerifiedEmailAddresses() ([]string, error) {
	input := &sesv2.ListEmailIdentitiesInput{}

	result, err := s.client.ListEmailIdentities(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to list verified email addresses: %w", err)
	}

	emails := make([]string, 0, len(result.EmailIdentities))
	for _, identity := range result.EmailIdentities {
		if identity.IdentityType == types.IdentityTypeEmailAddress && identity.IdentityName != nil {
			emails = append(emails, *identity.IdentityName)
		}
	}

	return emails, nil
}

func (s *SESService) DeleteVerifiedEmailAddress(email string) error {
	input := &sesv2.DeleteEmailIdentityInput{
		EmailIdentity: aws.String(email),
	}

	_, err := s.client.DeleteEmailIdentity(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to delete verified email address: %w", err)
	}

	return nil
}
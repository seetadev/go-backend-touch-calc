package email

/*
   NOTE: Original AWS SES implementation
   ------------------------------------
   This block is intentionally commented out so the project can run
   without AWS credentials or aws-sdk-go-v2 modules. The live
   implementation below is a console-logging stub that keeps the
   same public API.

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

   func (s *SESService) SendEmail(from string, to string, message *Message) error {
   	return s.SendEmailToMultiple(from, []string{to}, message)
   }

   func (s *SESService) SendEmailToMultiple(from string, toAddresses []string, message *Message) error {
   	// (original AWS SES send implementation lived here)
   }

   // func (s *SESService) VerifyEmailAddress(email string) error         { ... }
   // func (s *SESService) ListVerifiedEmailAddresses() ([]string, error) { ... }
   // func (s *SESService) DeleteVerifiedEmailAddress(email string) error { ... }
*/

import (
	"log"
)

// Message represents an email message payload.
type Message struct {
	Subject  string
	BodyText string
	BodyHTML string
	Charset  string
}

// SESService is now a lightweight stub that logs emails to the console
// instead of sending them via AWS SES. This avoids any AWS dependency
// while keeping the same type name and methods.
type SESService struct{}

// NewSESService returns a stub SESService without requiring AWS config.
func NewSESService() (*SESService, error) {
	return &SESService{}, nil
}

// NewMessage creates a new Message with a default charset.
func NewMessage() *Message {
	return &Message{
		Charset: "UTF-8",
	}
}

// SendEmail logs a single-recipient email.
func (s *SESService) SendEmail(from string, to string, message *Message) error {
	return s.SendEmailToMultiple(from, []string{to}, message)
}

// SendEmailToMultiple logs the email instead of sending via AWS SES.
func (s *SESService) SendEmailToMultiple(from string, toAddresses []string, message *Message) error {
	log.Printf("[EMAIL STUB] From=%s To=%v Subject=%q Text=%q HTML=%q",
		from, toAddresses, message.Subject, message.BodyText, message.BodyHTML)
	return nil
}

// VerifyEmailAddress is a no-op in the stub implementation.
func (s *SESService) VerifyEmailAddress(email string) error {
	log.Printf("[EMAIL STUB] VerifyEmailAddress called for %s (no-op)", email)
	return nil
}

// ListVerifiedEmailAddresses returns an empty list in the stub implementation.
func (s *SESService) ListVerifiedEmailAddresses() ([]string, error) {
	log.Printf("[EMAIL STUB] ListVerifiedEmailAddresses called (returns empty list)")
	return []string{}, nil
}

// DeleteVerifiedEmailAddress is a no-op in the stub implementation.
func (s *SESService) DeleteVerifiedEmailAddress(email string) error {
	log.Printf("[EMAIL STUB] DeleteVerifiedEmailAddress called for %s (no-op)", email)
	return nil
}

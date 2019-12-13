package vault

import (
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/sts"
)

// AssumeRoleProvider retrieves temporary credentials using STS GetSessionToken
type SessionTokenProvider struct {
	credentials.Expiry
	StsClient       *sts.STS
	Sessions        *KeyringSessions
	MasterCreds     *credentials.Credentials
	CredentialsName string
	Duration        time.Duration
	Mfa
}

// Retrieve returns temporary credentials using STS GetSessionToken
func (p *SessionTokenProvider) Retrieve() (credentials.Value, error) {
	log.Println("Getting credentials with GetSessionToken")

	session, err := p.getSessionToken()
	if err != nil {
		return credentials.Value{}, err
	}

	p.SetExpiration(*session.Expiration, DefaultExpirationWindow)

	value := credentials.Value{
		AccessKeyID:     *session.AccessKeyId,
		SecretAccessKey: *session.SecretAccessKey,
		SessionToken:    *session.SessionToken,
	}

	log.Printf("Using session token %s, expires in %s", formatKeyForDisplay(*session.AccessKeyId), time.Until(*session.Expiration).String())
	return value, nil
}

func (p *SessionTokenProvider) createSessionToken() (*sts.Credentials, error) {
	log.Printf("Creating new session token for profile %s", p.CredentialsName)
	var err error

	input := &sts.GetSessionTokenInput{
		DurationSeconds: aws.Int64(int64(p.Duration.Seconds())),
	}

	if p.MfaSerial != "" {
		input.SerialNumber = aws.String(p.MfaSerial)
		input.TokenCode, err = p.GetMfaToken()
		if err != nil {
			return nil, err
		}
	}

	resp, err := p.StsClient.GetSessionToken(input)
	if err != nil {
		return nil, err
	}

	return resp.Credentials, nil
}

func (p *SessionTokenProvider) getSessionToken() (*sts.Credentials, error) {
	session, err := p.Sessions.Retrieve(p.CredentialsName, p.MfaSerial)
	if err != nil {
		// session lookup missed, we need to create a new one.
		session, err = p.createSessionToken()
		if err != nil {
			return nil, err
		}

		err = p.Sessions.Store(p.CredentialsName, p.MfaSerial, session)
		if err != nil {
			return nil, err
		}
	}

	return session, err
}

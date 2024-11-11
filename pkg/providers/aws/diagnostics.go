package awsprovider

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

func (p *AWSProvider) PrintDiagnostics(ctx context.Context) error {
	l := logger.Get()
	l.Info("=== AWS Configuration Diagnostics ===")

	// Print Environment Variables
	l.Info("Environment Variables:")
	envVars := []string{
		"AWS_PROFILE",
		"AWS_DEFAULT_PROFILE",
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_SDK_LOAD_CONFIG",
		"AWS_CONFIG_FILE",
		"AWS_SHARED_CREDENTIALS_FILE",
		"AWS_ROLE_ARN",
		"AWS_WEB_IDENTITY_TOKEN_FILE",
	}

	for _, env := range envVars {
		value := os.Getenv(env)
		if value != "" {
			if strings.Contains(strings.ToLower(env), "secret") ||
				strings.Contains(strings.ToLower(env), "token") ||
				strings.Contains(strings.ToLower(env), "key") {
				value = "[REDACTED]"
			}
			l.Infof("  %s: %s", env, value)
		}
	}

	// Print Provider Configuration
	l.Info("\nProvider Configuration:")
	l.Infof("  Account ID: %s", p.AccountID)
	l.Infof("  Region: %s", p.Region)

	// Print AWS Credentials
	l.Info("\nAWS Credentials:")
	if p.Config == nil {
		l.Error("  AWS Config is nil!")
	} else {
		creds, err := p.Config.Credentials.Retrieve(ctx)
		if err != nil {
			l.Errorf("  Failed to retrieve credentials: %v", err)
		} else {
			l.Infof("  Provider: %s", creds.Source)
			l.Infof("  Access Key ID: %s", maskString(creds.AccessKeyID))
			l.Info("  Secret Access Key: [REDACTED]")
			if creds.SessionToken != "" {
				l.Info("  Session Token: [REDACTED]")
			}
			l.Infof("  Expires: %v", creds.Expires)
		}
	}

	// Try to get caller identity
	l.Info("\nCaller Identity:")
	stsClient := sts.NewFromConfig(*p.Config)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		l.Errorf("  Failed to get caller identity: %v", err)
	} else {
		l.Infof("  Account: %s", *identity.Account)
		l.Infof("  UserId: %s", *identity.UserId)
		l.Infof("  ARN: %s", *identity.Arn)
	}

	// Print AWS SDK configuration
	l.Info("\nAWS SDK Configuration:")
	l.Infof("  Region Configured: %t", p.Config.Region != "")
	l.Infof("  RetryMode: %s", string(p.Config.RetryMode))

	// Check if we can access necessary services
	l.Info("\nService Access Check:")

	// Check S3
	s3Client := s3.NewFromConfig(*p.Config)
	_, err = s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	l.Infof("  S3 Access: %v", err == nil)
	if err != nil {
		l.Errorf("    Error: %v", err)
	}

	ssmClient := ssm.NewFromConfig(*p.Config)
	_, err = ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String("/cdk-bootstrap/hnb659fds/version"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "ParameterNotFound") {
			l.Info(
				"  SSM Access: true (but CDK bootstrap parameters not found - environment needs bootstrapping)",
			)
		} else {
			l.Infof("  SSM Access: false")
			l.Errorf("    Error: %v", err)
		}
	} else {
		l.Infof("  SSM Access: true (CDK bootstrap parameters found)")
	}

	l.Info("=== End of AWS Configuration Diagnostics ===\n")
	return nil
}

// Helper function to mask sensitive strings
func maskString(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-4)
}

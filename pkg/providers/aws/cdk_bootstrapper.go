package awsprovider

import (
	"context"
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsssm"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// BootstrapEnvironment represents the environment to bootstrap
type BootstrapEnvironment struct {
	region    string
	accountID string
	qualifier string
}

// BootstrapStack defines the bootstrap stack
type BootstrapStack struct {
	awscdk.Stack
	qualifier string
}

func NewBootstrapStack(
	scope constructs.Construct,
	id string,
	qualifier string,
	props *awscdk.StackProps,
) *BootstrapStack {
	stack := &BootstrapStack{qualifier: qualifier}
	stackID := fmt.Sprintf("%s-%s", id, qualifier)
	awscdk.NewStack_Override(stack, scope, &stackID, props)

	// Create bucket with qualified name
	bucketName := fmt.Sprintf("cdk-%s-assets-%s-%s",
		qualifier,
		*props.Env.Account,
		*props.Env.Region,
	)

	// Create KMS key
	key := awskms.NewKey(stack, jsii.String("FileAssetsBucketEncryptionKey"), &awskms.KeyProps{
		Alias:             jsii.String(fmt.Sprintf("alias/cdk-%s-key", qualifier)),
		EnableKeyRotation: jsii.Bool(true),
		RemovalPolicy:     awscdk.RemovalPolicy_RETAIN,
	})

	// Create S3 bucket with encryption
	bucket := awss3.NewBucket(stack, jsii.String("StagingBucket"), &awss3.BucketProps{
		BucketName:        jsii.String(bucketName),
		Encryption:        awss3.BucketEncryption_KMS,
		EncryptionKey:     key,
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		Versioned:         jsii.Bool(true),
		RemovalPolicy:     awscdk.RemovalPolicy_RETAIN,
	})

	// Create qualified SSM parameters
	awsssm.NewStringParameter(
		stack,
		jsii.String("StagingBucketParameter"),
		&awsssm.StringParameterProps{
			ParameterName: jsii.String(fmt.Sprintf("/cdk-bootstrap/%s/bucket-name", qualifier)),
			StringValue:   bucket.BucketName(),
		},
	)

	awsssm.NewStringParameter(stack, jsii.String("BootstrapVersion"), &awsssm.StringParameterProps{
		ParameterName: jsii.String(fmt.Sprintf("/cdk-bootstrap/%s/version", qualifier)),
		StringValue:   jsii.String("14"),
	})

	// Create bucket policy
	bucket.AddToResourcePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("s3:*"),
		},
		Resources: &[]*string{
			bucket.BucketArn(),
			jsii.String(*bucket.BucketArn() + "/*"),
		},
		Principals: &[]awsiam.IPrincipal{
			awsiam.NewAccountPrincipal(jsii.String(*props.Env.Account)),
		},
	}))

	return stack
}

// isBootstrapped checks if the environment is already bootstrapped
func isBootstrapped(env BootstrapEnvironment) (bool, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(env.region),
	)
	if err != nil {
		return false, fmt.Errorf("unable to load SDK config: %w", err)
	}

	ssmClient := ssm.NewFromConfig(cfg)

	// Use qualified parameter name
	paramName := fmt.Sprintf("/cdk-bootstrap/%s/version", env.qualifier)
	_, err = ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name: &paramName,
	})

	if err != nil {
		return false, nil
	}

	return true, nil
}

// EnsureBootstrapped ensures the environment is bootstrapped
func EnsureBootstrapped(env BootstrapEnvironment) error {
	bootstrapped, err := isBootstrapped(env)
	if err != nil {
		return fmt.Errorf("failed to check bootstrap status: %w", err)
	}

	if !bootstrapped {
		app := awscdk.NewApp(nil)

		stackProps := &awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String(env.accountID),
				Region:  jsii.String(env.region),
			},
		}

		NewBootstrapStack(app, "CDKToolkit", env.qualifier, stackProps)
		app.Synth(nil)
	}

	return nil
}

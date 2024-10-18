package awsprovider

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewAWSProvider(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestCreateDeployment(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)

	err = provider.CreateDeployment(context.Background(), EC2Instance)
	assert.NoError(t, err)
}

func TestTerminateDeployment(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)

	err = provider.TerminateDeployment(context.Background())
	assert.NoError(t, err)
}

func TestListInstances(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)

	instances, err := provider.ListDeployments(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, instances)
}

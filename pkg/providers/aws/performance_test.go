//go:build integration
// +build integration

package awsprovider

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerformanceCreateInfrastructure(t *testing.T) {
	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Measure VPC creation time
	start := time.Now()
	err = provider.CreateVpc(ctx)
	duration := time.Since(start)
	assert.NoError(t, err)

	// VPC creation should typically complete within 2 minutes
	assert.Less(t, duration, 2*time.Minute)

	// Measure complete infrastructure creation time
	start = time.Now()
	err = provider.CreateInfrastructure(ctx)
	duration = time.Since(start)
	assert.NoError(t, err)

	// Full infrastructure creation should complete within 5 minutes
	assert.Less(t, duration, 5*time.Minute)

	// Cleanup
	err = provider.Destroy(ctx)
	assert.NoError(t, err)
}

func TestPerformanceResourcePolling(t *testing.T) {
	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create infrastructure first
	err = provider.CreateInfrastructure(ctx)
	require.NoError(t, err)

	// Measure polling performance
	start := time.Now()
	err = provider.StartResourcePolling(ctx)
	assert.NoError(t, err)

	// Let polling run for a minute
	time.Sleep(1 * time.Minute)

	// Check memory usage and CPU stats
	// TODO: Add metrics collection

	// Cleanup
	err = provider.Destroy(ctx)
	assert.NoError(t, err)
}

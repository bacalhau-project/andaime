package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDisplay(t *testing.T) {
	d := NewDisplay()
	assert.NotNil(t, d)
	assert.NotNil(t, d.LogBox)
	assert.NotNil(t, d.App)
	assert.NotNil(t, d.Table)
}

func TestNewDisplayInternal(t *testing.T) {
	d := NewDisplay()
	assert.NotNil(t, d)
	assert.NotNil(t, d.LogBox)
	assert.NotNil(t, d.App)
	assert.NotNil(t, d.Table)
}

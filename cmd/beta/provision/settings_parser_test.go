package provision_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bacalhau-project/andaime/cmd/beta/provision"
	"github.com/stretchr/testify/suite"
)

type SettingsParserTestSuite struct {
	suite.Suite
	parser *provision.SettingsParser
	tmpDir string
}

func (s *SettingsParserTestSuite) SetupTest() {
	s.parser = provision.NewSettingsParser()
	s.tmpDir = s.T().TempDir()
}

func (s *SettingsParserTestSuite) TestParseSettings() {
	tests := []struct {
		name           string
		fileContent    string
		expectedError  string
		expectedCount  int
		expectedValues map[string]string
	}{
		{
			name:           "empty file",
			fileContent:    "",
			expectedCount:  0,
			expectedValues: map[string]string{},
		},
		{
			name:          "valid settings",
			fileContent:   "key1:value1\nkey2:value2",
			expectedCount: 2,
			expectedValues: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:          "malformed setting",
			fileContent:   "invalid_line",
			expectedError: "invalid format: missing key-value separator ':', content: invalid_line",
		},
		{
			name:          "comment line",
			fileContent:   "# This is a comment\nkey: value",
			expectedCount: 1,
			expectedValues: map[string]string{
				"key": "value",
			},
		},
		{
			name:          "missing value",
			fileContent:   "key:",
			expectedError: "invalid value for key \"key\": empty value",
		},
		{
			name:          "duplicate keys",
			fileContent:   "key: value1\nkey: value2",
			expectedCount: 2,
			expectedValues: map[string]string{
				"key": "value1",
			},
		},
		{
			name:          "special characters in keys and values (dash, underscore, period)",
			fileContent:   "key-with-dash: value-with-dash\nkey.with.period: value.with.period\nkey_with_underscore: value_with_underscore",
			expectedCount: 3,
			expectedValues: map[string]string{
				"key_with_underscore": "value_with_underscore",
				"key-with-dash":       "value-with-dash",
				"key.with.period":     "value.with.period",
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tmpFile := filepath.Join(s.tmpDir, "settings.conf")
			err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644)
			s.Require().NoError(err)

			settings, err := s.parser.ParseFile(tmpFile)

			if tt.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedError)
				return
			}

			s.NoError(err)
			s.Equal(tt.expectedCount, len(settings))

			for key, expectedValue := range tt.expectedValues {
				found := false
				for _, setting := range settings {
					if setting.Key == key {
						s.Equal(expectedValue, setting.Value)
						found = true
						break
					}
				}
				s.True(found, "Setting %s not found", key)
			}
		})
	}
}

func TestSettingsParserSuite(t *testing.T) {
	suite.Run(t, new(SettingsParserTestSuite))
}

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
		// ... existing test cases ...
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

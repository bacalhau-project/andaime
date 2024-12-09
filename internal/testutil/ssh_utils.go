package testutil

import (
	"os"

	"github.com/bacalhau-project/andaime/internal/testdata"
)

// CreateSSHPublicPrivateKeyPairOnDisk creates temporary SSH key files for testing purposes.
// It writes predefined SSH key material to temporary files and returns their paths along with cleanup functions.
//
// Returns:
//   - string: Path to the public key file
//   - func(): Cleanup function for the public key file
//   - string: Path to the private key file
//   - func(): Cleanup function for the private key file
//
// The caller is responsible for calling both cleanup functions to remove the temporary files
// when they are no longer needed. Typically, this should be done using defer:
//
//	publicKeyPath, cleanupPublic, privateKeyPath, cleanupPrivate := CreateSSHPublicPrivateKeyPairOnDisk()
//	defer cleanupPublic()
//	defer cleanupPrivate()
func CreateSSHPublicPrivateKeyPairOnDisk() (string, func(), string, func()) {
	testSSHPublicKeyPath, cleanupPublicKey, err := WriteStringToTempFile(
		testdata.TestPublicSSHKeyMaterial,
	)
	if err != nil {
		panic(err)
	}
	testSSHPrivateKeyPath, cleanupPrivateKey, err := WriteStringToTempFile(
		testdata.TestPrivateSSHKeyMaterial,
	)
	if err != nil {
		panic(err)
	}

	return testSSHPublicKeyPath, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey
}

func WriteStringToTempFileWithExtension(content string, extension string) (string, func(), error) {
	path, cleanup, err := WriteStringToTempFile(content)
	if err != nil {
		return "", nil, err
	}

	pathPlusExtension := path + extension
	// Rename the file to add the extension
	err = os.Rename(path, pathPlusExtension)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return pathPlusExtension, cleanup, nil
}

func WriteStringToTempFile(content string) (string, func(), error) {
	tempFile, err := os.CreateTemp("", "temp-*")
	if err != nil {
		return "", nil, err
	}

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", nil, err
	}

	tempFile.Close()

	cleanup := func() {
		os.Remove(tempFile.Name())
	}

	return tempFile.Name(), cleanup, nil
}

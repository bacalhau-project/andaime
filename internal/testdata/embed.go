package testdata

import _ "embed"

//go:embed dummy_keys/id_ed25519.pub
var TestPublicSSHKeyMaterial string

//go:embed dummy_keys/id_ed25519
var TestPrivateSSHKeyMaterial string

//go:embed configs/azure.yaml
var TestAzureConfig string

//go:embed configs/config.yaml
var TestGenericConfig string

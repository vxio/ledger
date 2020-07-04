package config

import (
	"os"
	"path/filepath"
)

// file references for authentication and authorization between client and server
var (
	// generated certificate authority
	CAFile = configFile("ca.pem")
	// Server certificate and key
	ServerCertFile = configFile("server.pem")
	ServerKeyFile  = configFile("server-key.pem")
	// Unauthorized/unknown client certificate and key
	NobodyClientCertFile = configFile("nobody-client.pem")
	NobodyClientKeyFile  = configFile("nobody-client-key.pem")
	// Authorized/trusted client certificate and key
	RootClientCertFile = configFile("root-client.pem")
	RootClientKeyFile  = configFile("root-client-key.pem")
	// access control lists
	ACLModelFile  = configFile("model.conf")
	ACLPolicyFile = configFile("policy.csv")
)

func configFile(filename string) string {
	dir := os.Getenv("CONFIG_DIR")
	if dir != "" {
		return filepath.Join(dir, filename)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return filepath.Join(homeDir, ".ledger", filename)
}

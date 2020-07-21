package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"ledger/internal/agent"
	"ledger/internal/web"
)

func main() {
	cli := &cli{}

	cmd := &cobra.Command{
		Use:     "ledger: a distributed transaction journal",
		PreRunE: cli.setupConfig,
		RunE:    cli.run,
	}

	var err error

	err = setupFlags(cmd)
	if err != nil {
		log.Fatal(err)
	}

	err = cmd.Execute()
	if err != nil {
		log.Fatal(err)
	}

}

type cli struct {
	cfg cfg
}

// Reads the config fields from flags or a file and setups the agent's config
func (c *cli) setupConfig(cmd *cobra.Command, args []string) error {
	var err error

	configFile, err := cmd.Flags().GetString("config-file")
	if err != nil {
		return err
	}

	viper.SetConfigFile(configFile)

	if err = viper.ReadInConfig(); err != nil {
		// allow non-existent config file 
		if !errors.As(err, &viper.ConfigFileNotFoundError{}) {
			return err
		}
	}

	config := c.cfg

	config.DataDir = viper.GetString("data-dir")
	config.NodeName = viper.GetString("node-name")

	tcpAddr, err := net.ResolveTCPAddr("tcp", viper.GetString("bind-addr"))
	if err != nil {
		return err
	}
	config.BindAddr = tcpAddr
	config.RPCPort = viper.GetInt("rpc-port")
	config.StartJoinAddrs = viper.GetStringSlice("start-join-addrs")
	config.Bootstrap = viper.GetBool("bootstrap")
	config.ACLModelFile = viper.GetString("acl-model-file")
	config.ACLPolicyFile = viper.GetString("acl-policy-file")

	config.ServerTLSConfig.CAFile = viper.GetString("server-tls-ca-file")
	config.ServerTLSConfig.CertFile = viper.GetString("server-tls-cert-file")
	config.ServerTLSConfig.KeyFile = viper.GetString("server-tls-key-file")

	config.PeerTLSConfig.CAFile = viper.GetString("peer-tls-ca-file")
	config.PeerTLSConfig.CertFile = viper.GetString("peer-tls-cert-file")
	config.PeerTLSConfig.KeyFile = viper.GetString("peer-tls-key-file")

	if config.ServerTLSConfig.CertFile != "" && config.ServerTLSConfig.KeyFile != "" {
		config.ServerTLSConfig.Server = true
		if config.Config.ServerTLSConfig, err = web.SetupTLSConfig(config.ServerTLSConfig); err != nil {
			return err
		}
	}

	if config.PeerTLSConfig.CertFile != "" && config.ServerTLSConfig.KeyFile != "" {
		if config.Config.PeerTLSConfig, err = web.SetupTLSConfig(config.PeerTLSConfig); err != nil {
			return err
		}
	}

	return nil
}

func (c *cli) run(cmd *cobra.Command, args []string) error {
	var err error

	agent, err := agent.New(c.cfg.Config)
	if err != nil {
		return err
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc // block until the OS terminates the program
	return agent.Shutdown()
}

type cfg struct {
	agent.Config
	ServerTLSConfig web.TLSConfig
	PeerTLSConfig   web.TLSConfig
}

func setupFlags(cmd *cobra.Command) error {
	fs := cmd.Flags()

	fs.String("config-file", "", "Path to config file")

	dataDir := path.Join(os.TempDir(), "ledger")
	fs.String("data-dir", dataDir, "Directory to store log and Raft data")

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	fs.String("node-name", hostname, "Unique server ID")
	rpcPort := 8300
	serfPort := 8301

	fs.Int("rpc-port", rpcPort, "Port for RPC clients and Raft connections")
	fs.String("bind-addr", fmt.Sprintf("127.0.0.1:%d", serfPort), "Server address for Serf")
	fs.StringSlice("start-join-addrs", nil, "Serf address to join")
	fs.Bool("bootstrap", false, "Bootstrap the cluster")
	fs.String("acl-model-file", "", "Path to ACL model")
	fs.String("acl-policy-file", "", "Path to ACL policy")
	fs.String("server-tls-cert-file", "", "Path to server tls cert")
	fs.String("server-tls-key-file", "", "Path to server tls key")
	fs.String("server-tls-ca-file", "", "Path to server certificate authority")

	fs.String("peer-tls-cert-file", "", "Path to peer tls cert")
	fs.String("peer-tls-key-file", "", "Path to peer tls key")
	fs.String("peer-tls-ca-file", "", "Path to peer certificate authority")

	return viper.BindPFlags(cmd.Flags())
}

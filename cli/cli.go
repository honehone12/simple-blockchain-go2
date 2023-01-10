package cli

import (
	"flag"
	"fmt"
	"os"
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/clients"
	"simple-blockchain-go2/fullnode"
	"simple-blockchain-go2/genesis"
)

func printUsage() {
	fmt.Println()
	fmt.Println("usage:")
	fmt.Println(" node -n name -s SyncPort -c ConsensusPort -r RpcPort")
	fmt.Println("  genesis -n name")
	fmt.Println(" client")
	fmt.Println("  new -n Name")
	fmt.Println("  balance -n Name -p Port")
	fmt.Println("  airdrop -n Name -a Amount -p Port")
	fmt.Println("  transfer -n Name -a Amount -t To -p Port")
}

func validateArgs() {
	if len(os.Args) < 2 {
		exit()
	}
}

func exit() {
	printUsage()
	os.Exit(0)
}

func Run() error {
	validateArgs()

	nodeCmd := flag.NewFlagSet("node", flag.ExitOnError)
	nodeName := nodeCmd.String("n", "mynode", "node name")
	nodeSyncPort := nodeCmd.String("s", "3000", "port number to sync")
	nodeConsPort := nodeCmd.String("c", "5000", "port number to consensus")
	nodeRpcPort := nodeCmd.String("r", "8000", "port number to rpc")

	genesisCmd := flag.NewFlagSet("genesis", flag.ExitOnError)
	genesisName := genesisCmd.String("n", "mynode", "genesis node name")

	clientNewCmd := flag.NewFlagSet("newclient", flag.ExitOnError)
	clientNewName := clientNewCmd.String("n", "default", "account name")

	balanceCmd := flag.NewFlagSet("balance", flag.ExitOnError)
	balanceName := balanceCmd.String("n", "default", "account name")
	balancePort := balanceCmd.String("p", "8000", "port to send call")

	airdropCmd := flag.NewFlagSet("airdrop", flag.ExitOnError)
	airdropName := airdropCmd.String("n", "default", "account name")
	airdropAmount := airdropCmd.Uint64("a", 1, "airdrop amount")
	airdropPort := airdropCmd.String("p", "8000", "port to send call")

	fmt.Println()
	switch os.Args[1] {
	case "node":
		switch os.Args[2] {
		case "genesis":
			err := genesisCmd.Parse(os.Args[3:])
			if err != nil {
				exit()
			}
			gen, err := genesis.NewGenerator(*genesisName)
			if err != nil {
				return err
			}
			err = gen.Generate()
			if err != nil {
				return err
			} else {
				os.Exit(0)
			}
		default:
			err := nodeCmd.Parse(os.Args[2:])
			if err != nil {
				exit()
			}
			n, err := fullnode.NewFullNode(
				*nodeName,
				*nodeSyncPort,
				*nodeConsPort,
				*nodeRpcPort,
			)
			if err != nil {
				return err
			}
			return n.Run()
		}
	case "client":
		switch os.Args[2] {
		case "new":
			err := clientNewCmd.Parse(os.Args[3:])
			if err != nil {
				exit()
			}
			_, err = wallets.NewWallet(*clientNewName)
			return err
		case "balance":
			err := balanceCmd.Parse(os.Args[3:])
			if err != nil {
				exit()
			}
			c, err := clients.NewClient(*balanceName)
			if err != nil {
				return err
			}
			state, err := c.GetAccountInfo(*balancePort)
			if err != nil {
				return err
			}
			fmt.Printf("%d\n", state.Balance)
			return nil
		case "airdrop":
			err := airdropCmd.Parse(os.Args[3:])
			if err != nil {
				exit()
			}
			c, err := clients.NewClient(*airdropName)
			if err != nil {
				return err
			}
			res, err := c.Airdrop(*airdropAmount, *airdropPort)
			fmt.Println(res)
			return err
		default:
		}
	default:
	}
	return nil
}

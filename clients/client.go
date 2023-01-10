package clients

import (
	"encoding/json"
	"fmt"
	"net"
	"simple-blockchain-go2/accounts"
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/rpc"
	"simple-blockchain-go2/txs"
)

type Client struct {
	name   string
	wallet *wallets.Wallet
}

func NewClient(name string) (*Client, error) {
	w, err := wallets.NewWallet(name)
	if err != nil {
		return nil, err
	}
	return &Client{
		name:   name,
		wallet: w,
	}, nil
}

func (c *Client) Airdrop(amount uint64, port string) (string, error) {
	// TODO:
	// info can be cached in wallet
	info, err := c.GetAccountInfo(port)
	if err != nil {
		return "", err
	}
	c.wallet.Init(info.Nonce, info.Balance)

	param := rpc.AirdropParam{
		Amount: amount,
	}
	encPara, err := json.Marshal(param)
	if err != nil {
		return "", err
	}
	call := rpc.NewCall(rpc.Airdrop, encPara)
	encCall, err := json.Marshal(call)
	if err != nil {
		return "", err
	}
	tx := txs.NewTransaction(encCall)
	err = c.wallet.Sign(&tx)
	if err != nil {
		return "", err
	}

	res, err := c.sendTransaction(port, tx)
	if err != nil {
		return "", nil
	}
	return string(res), nil
}

func (c *Client) Transfer(amount uint64, to string, port string) (string, error) {
	return "", nil
}

func (c *Client) GetAccountInfo(port string) (*accounts.AccountState, error) {
	call := rpc.NewCall(rpc.GetAccountState, c.wallet.PublicKey())
	res, err := c.sendCall(port, call)
	if err != nil {
		return nil, err
	}
	state := &accounts.AccountState{}
	err = json.Unmarshal(res, state)
	return state, err
}

func (c *Client) sendCall(port string, call rpc.Call) ([]byte, error) {
	enc, err := json.Marshal(call)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 0, len(enc)+1)
	data = append(data, rpc.UnsignedRpc)
	data = append(data, enc...)
	return c.send(port, data)
}

func (c *Client) sendTransaction(port string, tx txs.Transaction) ([]byte, error) {
	enc, err := json.Marshal(&tx)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 0, len(enc)+1)
	data = append(data, rpc.SifnedRpc)
	data = append(data, enc...)
	return c.send(port, data)
}

func (c *Client) send(port string, data []byte) ([]byte, error) {
	ip4 := fmt.Sprintf(common.Ip4Format, port)
	conn, err := net.Dial(common.Tcp, ip4)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	dataLen := len(data)
	prefixedData, err := common.ToHex(int32(dataLen))
	if err != nil {
		return nil, err
	}
	prefixedData = append(prefixedData, data...)
	_, err = conn.Write(prefixedData)
	if err != nil {
		return nil, err
	}

	buff := make([]byte, 4)
	_, err = conn.Read(buff)
	if err != nil {
		return nil, err
	}
	resLen, err := common.FromHex[int32](buff)
	if err != nil {
		return nil, err
	}
	buff = make([]byte, resLen)
	_, err = conn.Read(buff)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

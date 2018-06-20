package models

import (
	"log"
	"net/http"
	"time"

	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/SmartMeshFoundation/SmartRaiden/params"
	"github.com/go-errors/errors"
)

// RaidenNode a smartraiden node
type RaidenNode struct {
	Host          string
	Address       string
	Name          string
	APIAddress    string
	ListenAddress string
	ConditionQuit *params.ConditionQuit
	DebugCrash    bool
}

// Start start a raiden node
func (node *RaidenNode) Start(env *TestEnv) {
	logfile := fmt.Sprintf("./log/%s.log", env.CaseName+"-"+node.Name)
	go ExecShell(env.Main, node.getParamStr(env), logfile, true)

	count := 0
	t := time.Now()
	for !node.IsRunning() {
		time.Sleep(time.Second * 3)
		count++
		if count > 40 {
			Logger.Printf("NODE %s %s start with %s TIMEOUT\n", node.Address, node.Host, node.ConditionQuit.QuitEvent)
			panic("Start raiden node TIMEOUT")
		}
	}
	used := time.Since(t)
	if node.DebugCrash {
		Logger.Printf("NODE %s %s start with %s in %fs", node.Address, node.Host, node.ConditionQuit.QuitEvent, used.Seconds())
	} else {
		Logger.Printf("NODE %s %s start in %fs", node.Address, node.Host, used.Seconds())
	}
}

func (node *RaidenNode) getParamStr(env *TestEnv) []string {
	var param []string
	param = append(param, "--datadir="+env.DataDir)
	param = append(param, "--api-address="+node.APIAddress)
	param = append(param, "--listen-address="+node.ListenAddress)
	param = append(param, "--address="+node.Address)
	param = append(param, "--keystore-path="+env.KeystorePath)
	param = append(param, "--registry-contract-address="+env.RegistryContractAddress)
	param = append(param, "--password-file="+env.PasswordFile)
	param = append(param, "--xmpp-server="+env.XMPPServer)
	param = append(param, "--eth-rpc-endpoint="+env.EthRPCEndpoint)
	param = append(param, fmt.Sprintf("--verbosity=%d", env.Verbosity))
	if env.Debug == true {
		param = append(param, "--debug")
	}
	if node.DebugCrash == true {
		buf, err := json.Marshal(node.ConditionQuit)
		if err != nil {
			panic(err)
		}
		param = append(param, "--debugcrash")
		param = append(param, "--conditionquit="+string(buf))
	}
	return param
}

// StartWithConditionQuit start a raiden node whit condition quit
func (node *RaidenNode) StartWithConditionQuit(env *TestEnv, c *params.ConditionQuit) {
	node.ConditionQuit = c
	node.DebugCrash = true
	node.Start(env)
}

// IsRunning check by api address
func (node *RaidenNode) IsRunning() bool {
	req := &Req{
		FullURL: node.Host + "/api/1/address",
		Method:  http.MethodGet,
		Payload: "",
		Timeout: time.Second * 30,
	}
	statusCode, _, err := req.Invoke()
	if err != nil {
		return false
	}
	if statusCode != 200 {
		Logger.Printf("Exception response:%d\n", statusCode)
		panic("Exception response")
	}
	return true
}

// TransferPayload API  http body
type TransferPayload struct {
	Amount   int32 `json:"amount"`
	Fee      int64 `json:"fee"`
	IsDirect bool  `json:"is_direct"`
}

// SendTrans send a transfer
func (node *RaidenNode) SendTrans(tokenAddress string, amount int32, targetAddress string, isDirect bool) error {
	p, _ := json.Marshal(TransferPayload{
		Amount:   amount,
		Fee:      0,
		IsDirect: isDirect,
	})
	req := &Req{
		FullURL: node.Host + "/api/1/transfers/" + tokenAddress + "/" + targetAddress,
		Method:  http.MethodPost,
		Payload: string(p),
		Timeout: time.Second * 180,
	}
	statusCode, _, err := req.Invoke()
	if err != nil {
		return err
	}
	if statusCode != 200 {
		return errors.New(statusCode)
	}
	return nil
}

// ExecShell : run shell commands
func ExecShell(cmdstr string, param []string, logfile string, canquit bool) bool {
	var err error
	/* #nosec */
	cmd := exec.Command(cmdstr, param...)

	stdout, err := cmd.StdoutPipe()
	stderr, err := cmd.StderrPipe()

	err = cmd.Start()
	if err != nil {
		log.Println(err)
		return false
	}

	reader := bufio.NewReader(stdout)
	readererr := bufio.NewReader(stderr)

	logFile, err := os.Create(logfile)
	defer logFile.Close()
	if err != nil {
		log.Fatalln("Create log file error !", logfile)
	}

	debugLog := log.New(logFile, "", 0)
	//A real-time loop reads a line in the output stream.
	go func() {
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil || io.EOF == readErr {
				break
			}
			//log.Println(line)
			debugLog.Println(line)
		}
	}()

	//go func() {
	for {
		line, readErr := readererr.ReadString('\n')
		if readErr != nil || io.EOF == readErr {
			break
		}
		//log.Println(line)
		debugLog.Println(line)
	}
	//}()

	err = cmd.Wait()

	if !canquit {
		log.Println("cmd ", cmdstr, " exited:", param)
	}

	if err != nil {
		//log.Println(err)
		debugLog.Println(err)
		if !canquit {
			os.Exit(-1)
		}
		return false
	}
	if !canquit {
		os.Exit(-1)
	}
	return true
}

package models

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aeternity/aepp-sdk-go/account"
	"github.com/aeternity/aepp-sdk-go/aeternity"
	"github.com/aeternity/aepp-sdk-go/binary"
	"github.com/aeternity/aepp-sdk-go/config"
	"github.com/aeternity/aepp-sdk-go/naet"
	"github.com/aeternity/aepp-sdk-go/swagguard/node/models"
	"github.com/aeternity/aepp-sdk-go/transactions"
	"github.com/astaxie/beego"
	"github.com/tyler-smith/go-bip39"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"
)

var NodeURL = beego.AppConfig.String("NodeURL")
var CompilerURL = beego.AppConfig.String("CompilerURL")
var NodeURLTestNet =beego.AppConfig.String("NodeURLTestNet")


//===================================================================================================================================================================================================
//|                           															AE-BASE																										 |
///===================================================================================================================================================================================================

//根据助记词返回用户
func MnemonicAccount(mnemonic string, addressIndex uint32) (acc *account.Account, m string, error error) {

	//生成种子
	seed, err := account.ParseMnemonic(mnemonic)
	if err != nil {
		return nil, mnemonic, err
	}

	//验证助记词
	_, err = bip39.EntropyFromMnemonic(mnemonic)

	if err != nil {
		return nil, mnemonic, err
	}

	//获取子账户
	// Derive the subaccount m/44'/457'/3'/0'/1'
	key, err := account.DerivePathFromSeed(seed, 0, addressIndex-1)
	if err != nil {
		return nil, mnemonic, err
	}

	// 生成账户
	alice, err := account.BIP32KeyToAeKey(key)
	if err != nil {
		return nil, mnemonic, err
	}
	return alice, mnemonic, nil
}

//根据私钥返回用户
func SigningKeyHexStringAccount(signingKey string) (*account.Account, error) {
	acc, e := account.FromHexString(signingKey)
	return acc, e
}

//随机创建用户
func CreateAccount() (*account.Account, string) {
	mnemonic, signingKey, _ := CreateAccountUtils()
	acc, _ := account.FromHexString(signingKey)
	return acc, mnemonic
}

//随机创建用户,返回助记词
func CreateAccountUtils() (mnemonic string, signingKey string, address string) {
	//创建助记词
	entropy, _ := bip39.NewEntropy(128)
	//生成助记词
	mne, _ := bip39.NewMnemonic(entropy)
	//生成种子
	seed, _ := account.ParseMnemonic(mne)
	//验证助记词
	_, _ = bip39.EntropyFromMnemonic(mne)
	//生成子账户
	key, _ := account.DerivePathFromSeed(seed, 0, 0)
	//获取账户
	alice, _ := account.BIP32KeyToAeKey(key)
	//返回私钥和信息
	return mne, alice.SigningKeyToHexString(), alice.Address
}

//返回最新区块高度
func ApiBlocksTop() (height uint64) {
	client := naet.NewNode(NodeURL, false)
	h, _ := client.GetHeight()
	return h
}

//地址信息返回用户信息和余额
func ApiGetAccount(address string) (account *models.Account, e error) {
	client := naet.NewNode(NodeURL, false)
	acc, e := client.GetAccount(address)
	return acc, e
}

//===================================================================================================================================================================================================
//|                           															AEX-9																										 |
///===================================================================================================================================================================================================

type CallInfoResult struct {
	CallInfo CallInfo `json:"call_info"`
}

type CallInfo struct {
	ReturnType  string `json:"return_type"`
	ReturnValue string `json:"return_value"`
}

//调用aex9 合约方法
func CallContractFunction(account *account.Account, ctID string, function string, args []string) (s interface{}, e error) {
	//获取节点信息
	n := naet.NewNode(NodeURL, false)
	//获取编译器信息
	c := naet.NewCompiler(CompilerURL, false)
	//创建上下文
	ctx := aeternity.NewContext(account, n)
	//关联编译器
	ctx.SetCompiler(c)
	//创建合约
	contract := aeternity.NewContract(ctx)
	//获取合约代码
	expected, _ := ioutil.ReadFile("contract/PriceFeedContract.aes")
	//调用合约代码
	callReceipt, err := contract.Call(ctID, string(expected), function, args, config.CompilerBackendFATE)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(NodeURL + "/v2/transactions/" + callReceipt.Hash + "/info")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	//获取合约调用信息
	//response := Get(NodeURL + "/v2/transactions/" + callReceipt.Hash + "/info")
	//解析jSON
	var callInfoResult CallInfoResult
	err = json.Unmarshal(body, &callInfoResult)
	if err != nil {
		return nil, err
	}
	//解析结果
	decodeResult, err := c.DecodeCallResult(callInfoResult.CallInfo.ReturnType, callInfoResult.CallInfo.ReturnValue, function, string(expected), config.Compiler.Backend)
	if err != nil {
		return nil, err
	}
	//返回结果
	return decodeResult, err
}
func Md5V(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

var cacheCallMap = make(map[string]string)
var cacheResultlMap = make(map[string]interface{})

func CallStaticContractFunction(address string, ctID string, function string, args []string) (s interface{}, functionEncode string, e error) {
	node := naet.NewNode(NodeURL, false)
	compile := naet.NewCompiler(CompilerURL, false)

	var source []byte
	source, _ = ioutil.ReadFile("contract/PriceFeedContract.aes")


	var callData = ""
	if v, ok := cacheCallMap[Md5V(function+"#"+address+"#"+ctID+"#"+fmt.Sprintf("%s", args))]; ok {
		if ok && len(v)>5{
			callData = v

		}else{
			data, err := compile.EncodeCalldata(string(source), function, args, config.CompilerBackendFATE)
			if err != nil {
				return nil, function, err
			}
			callData = data
			cacheCallMap[Md5V(function+"#"+address+"#"+ctID+"#"+fmt.Sprintf("%s", args))] = callData
		}

	} else {
		data, err := compile.EncodeCalldata(string(source), function, args, config.CompilerBackendFATE)
		if err != nil {
			return nil, function, err
		}
		callData = data

		cacheCallMap[Md5V(function+"#"+address+"#"+ctID+"#"+fmt.Sprintf("%s", args))] = callData
	}




	callTx, err := transactions.NewContractCallTx(address, ctID, big.NewInt(0), config.Client.Contracts.GasLimit, config.Client.GasPrice, config.Client.Contracts.ABIVersion, callData, transactions.NewTTLNoncer(node))
	if err != nil {
		return nil, function, err
	}

	w := &bytes.Buffer{}
	err = callTx.EncodeRLP(w)
	if err != nil {
		println(callTx.CallData)
		return nil, function, err
	}

	txStr := binary.Encode(binary.PrefixTransaction, w.Bytes())

	body := "{\"accounts\":[{\"pub_key\":\"" + address + "\",\"amount\":100000000000000000000000000000000000}],\"txs\":[{\"tx\":\"" + txStr + "\"}]}"

	response := PostBody(NodeURLTestNet+"/v2/debug/transactions/dry-run", body, "application/json")
	var tryRun TryRun
	err = json.Unmarshal([]byte(response), &tryRun)
	if err != nil {
		return nil, function, err
	}

	if v, ok := cacheResultlMap[Md5V(function+"#"+address+"#"+ctID+"#"+fmt.Sprintf("%s", args))+"#"+tryRun.Results[0].CallObj.ReturnValue]; ok {
		return v, function, err
	} else {
		decodeResult, err := compile.DecodeCallResult(tryRun.Results[0].CallObj.ReturnType, tryRun.Results[0].CallObj.ReturnValue, function, string(source), config.Compiler.Backend)
		cacheResultlMap[Md5V(function+"#"+address+"#"+ctID+"#"+fmt.Sprintf("%s", args))+"#"+tryRun.Results[0].CallObj.ReturnValue] = decodeResult
		return decodeResult, function, err
	}

}


//response:请求返回的内容
func Get(url string) (response string) {
	client := http.Client{Timeout: 60 * time.Second}
	resp, error := client.Get(url)
	defer resp.Body.Close()
	if error != nil {
		panic(error)
	}
	var buffer [512]byte
	result := bytes.NewBuffer(nil)
	for {
		n, err := resp.Body.Read(buffer[0:])
		result.Write(buffer[0:n])
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
	}
	response = result.String()
	return
}

//发送POST请求
//url:请求地址		data:POST请求提交的数据		contentType:请求体格式，如：application/json
//content:请求返回的内容
func Post(url string, data interface{}, contentType string) (content string) {
	jsonStr, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Add("content-type", contentType)
	if err != nil {
		panic(err)
	}
	defer req.Body.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, error := client.Do(req)
	if error != nil {
		panic(error)
	}
	defer resp.Body.Close()
	result, _ := ioutil.ReadAll(resp.Body)
	content = string(result)
	return
}

//发送POST请求
//url:请求地址		data:POST请求提交的数据		contentType:请求体格式，如：application/json
//content:请求返回的内容
func PostBody(url string, data string, contentType string) (content string) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(data)))
	req.Header.Add("content-type", contentType)
	if err != nil {
		panic(err)
	}
	defer req.Body.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, error := client.Do(req)
	if error != nil {
		panic(error)
	}
	defer resp.Body.Close()
	result, _ := ioutil.ReadAll(resp.Body)
	content = string(result)
	return
}


type TryRun struct {
	Results []Results `json:"results"`
}
type CallObj struct {
	CallerID    string        `json:"caller_id"`
	CallerNonce int           `json:"caller_nonce"`
	ContractID  string        `json:"contract_id"`
	GasPrice    int           `json:"gas_price"`
	GasUsed     int           `json:"gas_used"`
	Height      int           `json:"height"`
	Log         []interface{} `json:"log"`
	ReturnType  string        `json:"return_type"`
	ReturnValue string        `json:"return_value"`
}
type Results struct {
	CallObj CallObj `json:"call_obj"`
	Result  string  `json:"result"`
	Type    string  `json:"type"`
}


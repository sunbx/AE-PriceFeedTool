package main

import (
	"PriceFeedTool/models"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aeternity/aepp-sdk-go/utils"
	"github.com/astaxie/beego"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func SynAeBlock() {
	var publicKey = beego.AppConfig.String("PublicKey")
	var signingKey = beego.AppConfig.String("SigningKey")
	var dataProvider = beego.AppConfig.String("DataProvider")
	var oct = beego.AppConfig.String("PriceOracleContract")

	//fmt.Println("ORACLES QUERIES LIST DATA...")
	//fmt.Println("CHECK ORACLES", dataProvider)
	response := Get(models.NodeURL + "/v2/oracles/" + strings.Replace(oct, "ct_", "ok_", -1) + "/queries/")
	//解析区块信息为实体
	var oracleQueries OracleQueries
	err := json.Unmarshal([]byte(response), &oracleQueries)
	if err != nil {
		fmt.Println("ORACLES QUERIES LIST DATA", " JSON UNMARSHAL ERROR")
	}
	account, _ := models.SigningKeyHexStringAccount(signingKey)

	for i := 0; i < len(oracleQueries.OracleQueries); i++ {
		//fmt.Println("FIND THE DATA WAITING FOR A RESPONSE...")

		if "or_Xfbg4g==" == oracleQueries.OracleQueries[i].Response {

			//fmt.Println("CHECK TO SEE IF IT HAS BEEN RESPONDED")
			s, _, err := models.CallStaticContractFunction(publicKey, oct, "getIsMapPriceExist", []string{oracleQueries.OracleQueries[i].ID, publicKey})

			data, _ := json.Marshal(s)
			//fmt.Println(string(data))
			if err != nil {
				fmt.Println("CHECK TO SEE IF IT HAS BEEN RESPONDED ERROR")
			}
			if strings.Contains(string(data), publicKey) {
				//fmt.Println("CHECK SUCCESS", " HAS THE RESPONSE")
			} else {
				fmt.Println("FIND THE DATA WAITING FOR A RESPONSE", " SUCCESS OP-ID : "+oracleQueries.OracleQueries[i].ID)
				//fmt.Println("CHECK SUCCESS", " NO RESPONSE")

				//fmt.Println("ACCESS TO MARKET DATA...")

				price := getApiPrice(dataProvider)
				if price == 0 {
					continue
				}
				s, e := models.CallContractFunction(account, oct, "offerPrice", []string{oracleQueries.OracleQueries[i].ID, GetRealAebalanceBigInt(price).String()})
				if e != nil {
					fmt.Println("OFFER_PRICE ERROR : ", e.Error())
				}
				fmt.Println("OFFER_PRICE SUCCESS", s)
			}

			//fmt.Println("CHECK IF A RESPONSE IS REQUIRED ")
			s, _, err = models.CallStaticContractFunction(publicKey, oct, "getIsRespondStatus", []string{oracleQueries.OracleQueries[i].ID})
			data, _ = json.Marshal(s)
			//fmt.Println(string(data))
			if err != nil {
				fmt.Println("CHECK IF A RESPONSE IS REQUIRED - ERROR")
			}
			if strings.Contains(string(data), oracleQueries.OracleQueries[i].ID) {
				s, e := models.CallContractFunction(account, oct, "respond", []string{oracleQueries.OracleQueries[i].ID})
				if e != nil {
					fmt.Println("RESPOND ERROR : ", e.Error())
				}
				fmt.Println("RESPOND SUCCESS", s, oracleQueries.OracleQueries[i].ID)
				fmt.Println("")
			}
		}

	}

}

func getApiPrice(t string) float64 {
	if t == "Huobi" {
		response := Get("https://api.huobi.fm/market/detail/merged?symbol=aeusdt")
		var huobi Huobi
		err := json.Unmarshal([]byte(response), &huobi)
		if err != nil {
			fmt.Println("ACCESS TO MARKET DATA API ERROR ", err.Error())
		}
		fmt.Println("HUOBI - PRICE", huobi.Tick.Close)
		return huobi.Tick.Close
	}
	if t == "Coingecko" {
		response := Get("https://api.coingecko.com/api/v3/simple/price?ids=aeternity&vs_currencies=usd")
		var coingecko Coingecko
		err := json.Unmarshal([]byte(response), &coingecko)
		if err != nil {
			fmt.Println("ACCESS TO MARKET DATA API ERROR ", err.Error())
		}
		fmt.Println("COINGECKO - PRICE", coingecko.Aeternity.Usd)
		return coingecko.Aeternity.Usd
	}
	if t == "Gate" {
		response := Get("https://data.gateapi.io/api2/1/ticker/ae_usdt")
		var gate Gate
		err := json.Unmarshal([]byte(response), &gate)
		if err != nil {
			fmt.Println("ACCESS TO MARKET DATA API ERROR ", err.Error())
		}
		fmt.Println("GATE - PRICE", gate.Last)
		last, err := strconv.ParseFloat(gate.Last, 64)
		return last
	}
	return 0
}

type OracleQueries struct {
	OracleQueries []OracleQuery `json:"oracle_queries"`
}
type OracleQuery struct {
	Fee         utils.BigInt `json:"fee"`
	ID          string       `json:"id"`
	OracleID    string       `json:"oracle_id"`
	Query       string       `json:"query"`
	Response    string       `json:"response"`
	ResponseTTL TTL          `json:"response_ttl"`
	SenderID    *string      `json:"sender_id"`
	SenderNonce *uint64      `json:"sender_nonce"`
	TTL         *uint64      `json:"ttl"`
}

type TTL struct {
	Type  string `json:"type"`
	Value uint64 `json:"value"`
}

type Coingecko struct {
	Aeternity Aeternity `json:"aeternity"`
}
type Aeternity struct {
	Usd float64 `json:"usd"`
}

//=======================================

type Huobi struct {
	Status string `json:"status"`
	Ch     string `json:"ch"`
	Ts     int64  `json:"ts"`
	Tick   Tick   `json:"tick"`
}
type Tick struct {
	Amount  float64   `json:"amount"`
	Open    float64   `json:"open"`
	Close   float64   `json:"close"`
	High    float64   `json:"high"`
	ID      int64     `json:"id"`
	Count   int       `json:"count"`
	Low     float64   `json:"low"`
	Version int64     `json:"version"`
	Ask     []float64 `json:"ask"`
	Vol     float64   `json:"vol"`
	Bid     []float64 `json:"bid"`
}

//==========================
type Gate struct {
	QuoteVolume   string `json:"quoteVolume"`
	BaseVolume    string `json:"baseVolume"`
	HighestBid    string `json:"highestBid"`
	High24Hr      string `json:"high24hr"`
	Last          string `json:"last"`
	LowestAsk     string `json:"lowestAsk"`
	Elapsed       string `json:"elapsed"`
	Result        string `json:"result"`
	Low24Hr       string `json:"low24hr"`
	PercentChange string `json:"percentChange"`
}

func Get(url string) (response string) {
	client := http.Client{Timeout: 600 * time.Second}
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

func GetRealAebalanceBigInt(amount float64) *big.Int {
	newFloat := big.NewFloat(amount)
	basefloat := big.NewFloat(1000000000000000000)
	float1 := big.NewFloat(1)
	float1.Mul(newFloat, basefloat)
	resultAmount := new(big.Int)
	float1.Int(resultAmount)
	return resultAmount
}

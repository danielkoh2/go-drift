package priorityFee

import (
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/go-resty/resty/v2"
	"strings"
)

type DriftPriorityFeeLevels struct {
	Min         uint64 `json:"min"`
	Low         uint64 `json:"low"`
	Medium      uint64 `json:"medium"`
	High        uint64 `json:"high"`
	VeryHigh    uint64 `json:"veryHigh"`
	UnsafeMax   uint64 `json:"unsafeMax"`
	MarketType  string `json:"marketType"`
	MarketIndex uint16 `json:"marketIndex"`
}
type DriftPriorityFeeResponse []DriftPriorityFeeLevels

func FetchDriftPriorityFee(
	url string,
	marketTypes []string,
	marketIndexs []uint16,
) DriftPriorityFeeResponse {
	var marketIndexStrings []string
	for _, marketIndex := range marketIndexs {
		marketIndexStrings = append(marketIndexStrings, fmt.Sprintf("%d", marketIndex))
	}
	client := resty.New()
	resp, err := client.R().
		SetQueryParams(map[string]string{
			"marketType":  strings.Join(marketTypes, ","),
			"marketIndex": strings.Join(marketIndexStrings, ","),
		}).
		Get(fmt.Sprintf("%s/batchPriorityFees", url))
	if err == nil && resp != nil && resp.IsSuccess() {
		var response DriftPriorityFeeResponse
		err = json.Unmarshal(resp.Body(), &response)
		if err != nil {
			spew.Dump(err)
			return nil
		}
		return response
	}
	return []DriftPriorityFeeLevels{}
}

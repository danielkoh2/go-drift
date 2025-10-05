package events

import (
	"driftgo/lib/drift"
	"encoding/base64"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"regexp"
	"strings"
)

const DriftProgramId = "dRiftyHA39MWEi3m9aunc5MzRF1JYuBsbn6VPcn33UH"

const PROGRAM_LOG = "Program log: "

const PROGRAM_DATA = "Program data: "

const PROGRAM_LOG_START_INDEX = len(PROGRAM_LOG)
const PROGRAM_DATA_START_INDEX = len(PROGRAM_DATA)

func ParseLogs(
	logs []string,
	programId string,
) []*Event {
	var events []*Event
	execution := &ExecutionContext{}

	for _, log := range logs {
		if strings.HasPrefix(log, "Log truncated") {
			break
		}
		event, newProgram, didPop := handleLog(
			execution,
			log,
			programId,
		)
		if event != nil {
			events = append(events, event)
		}
		if newProgram != "" {
			execution.Push(newProgram)
		}
		if didPop {
			execution.Pop()
		}
	}
	return events
}

func handleLog(execution *ExecutionContext, log string, programId string) (*Event, string, bool) {
	if execution.Program() == programId {
		return handleProgramLog(log, programId)
	} else {
		newProgram, didPop := handleSystemLog(log, programId)
		return nil, newProgram, didPop
	}
}

func handleProgramLog(log string, programId string) (*Event, string, bool) {
	if strings.HasPrefix(log, PROGRAM_LOG) {
		logStr := log[PROGRAM_LOG_START_INDEX:]
		return parseEvent(logStr), "", false
	} else if strings.HasPrefix(log, PROGRAM_DATA) {
		logStr := log[PROGRAM_DATA_START_INDEX:]
		return parseEvent(logStr), "", false
	} else {
		newProgram, didPop := handleSystemLog(log, programId)
		return nil, newProgram, didPop
	}
}

func handleSystemLog(log string, programId string) (string, bool) {
	// System component.
	logStart := strings.Split(log, ":")[0]
	programStart := fmt.Sprintf("Program %s invoke", programId)
	// Did the program finish executing?
	matched, err := regexp.MatchString(`^Program (.*) success$`, logStart)
	if err == nil && matched {
		return "", true
		// Recursive call.
	}
	if strings.HasPrefix(logStart, programStart) {
		return programId, false
	}
	// CPI call.
	if strings.Contains(logStart, "invoke") {
		return "cpi", false // Any string will do.
	}
	return "", false
}

func parseEvent(log string) *Event {
	data, err := base64.StdEncoding.DecodeString(log)
	if err != nil {
		return &Event{
			Data:      log,
			EventType: EventTypeRaw,
		}
	} else {
		eventRecord, eventType := parseEventRecord(data)
		if eventRecord != nil {
			return &Event{
				Data:      eventRecord,
				EventType: eventType,
			}
		} else {
			return nil
		}
	}
}

func parseEventRecord(data []byte) (interface{}, EventType) {
	decoder := bin.NewBinDecoder(data)
	typeId, err := decoder.ReadTypeID()
	if err != nil {
		return nil, EventTypeRaw
	}
	decoder.Reset(data)
	switch typeId {
	case drift.Event_NewUserRecord:
		var record drift.NewUserRecord
		err = decoder.Decode(&record)
		return &record, EventTypeNewUserRecord
	case drift.Event_DepositRecord:
		var record drift.DepositRecord
		err = decoder.Decode(&record)
		return &record, EventTypeDepositRecord
	case drift.Event_SpotInterestRecord:
		var record drift.SpotInterestRecord
		err = decoder.Decode(&record)
		return &record, EventTypeSpotInterestRecord
	case drift.Event_FundingPaymentRecord:
		var record drift.FundingPaymentRecord
		err = decoder.Decode(&record)
		return &record, EventTypeFundingPaymentRecord
	case drift.Event_FundingRateRecord:
		var record drift.FundingRateRecord
		err = decoder.Decode(&record)
		return &record, EventTypeFundingRateRecord
	case drift.Event_CurveRecord:
		var record drift.CurveRecord
		err = decoder.Decode(&record)
		return &record, EventTypeCurveRecord
	case drift.Event_OrderRecord:
		var record drift.OrderRecord
		err = decoder.Decode(&record)
		return &record, EventTypeOrderRecord
	case drift.Event_OrderActionRecord:
		var record drift.OrderActionRecord
		err = decoder.Decode(&record)
		return &record, EventTypeOrderActionRecord
	case drift.Event_LpRecord:
		var record drift.LpRecord
		err = decoder.Decode(&record)
		return &record, EventTypeLPRecord
	case drift.Event_LiquidationRecord:
		var record drift.LiquidationRecord
		err = decoder.Decode(&record)
		return &record, EventTypeLiquidationRecord
	case drift.Event_SettlePnlRecord:
		var record drift.SettlePnlRecord
		err = decoder.Decode(&record)
		return &record, EventTypeSettlePnlRecord
	case drift.Event_InsuranceFundRecord:
		var record drift.InsuranceFundRecord
		err = decoder.Decode(&record)
		return &record, EventTypeInsuranceFundRecord
	case drift.Event_InsuranceFundStakeRecord:
		var record drift.InsuranceFundStakeRecord
		err = decoder.Decode(&record)
		return &record, EventTypeInsuranceFundStakeRecord
	case drift.Event_SwapRecord:
		var record drift.SwapRecord
		err = decoder.Decode(&record)
		return &record, EventTypeSwapRecord
	}
	return nil, EventTypeRaw
}

type ExecutionContext struct {
	stack []string
}

func (p *ExecutionContext) Program() string {
	if len(p.stack) == 0 {
		return ""
	}
	return p.stack[len(p.stack)-1]
}

func (p *ExecutionContext) Push(newProgram string) {
	p.stack = append(p.stack, newProgram)
}

func (p *ExecutionContext) Pop() {
	if len(p.stack) == 0 {
		return
	}
	p.stack = p.stack[:len(p.stack)-1]
}

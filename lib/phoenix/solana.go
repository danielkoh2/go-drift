package phoenix

import ag_binary "github.com/gagliardetto/binary"

type ClockData struct {
	Slot                uint64 `json:"slot"`
	EpochStartTime      int64  `json:"epochStartTime"`
	Epoch               uint64 `json:"epoch"`
	LeaderScheduleEpoch uint64 `json:"leaderScheduleEpoch"`
	UnixTimestamp       int64  `json:"unixTimestamp"`
}

func (obj ClockData) UnmarshalWithDecoder(decoder *ag_binary.Decoder) error {
	err := decoder.Decode(&obj.Slot)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.EpochStartTime)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.Epoch)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.LeaderScheduleEpoch)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.UnixTimestamp)
	if err != nil {
		return err
	}
	return nil
}

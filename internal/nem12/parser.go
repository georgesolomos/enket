package nem12

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

type Parser struct {
	logger *slog.Logger
	file   *os.File
}

func NewParser(logger *slog.Logger, file *os.File) *Parser {
	return &Parser{
		logger: logger,
		file:   file,
	}
}

func (p *Parser) Parse() (UsageData, error) {
	nemReader := p.createNemReader(p.file)
	hasHeader, err := p.checkNem12Header(nemReader)
	if err != nil {
		return nil, err
	}
	if !hasHeader {
		// Reset the file and recreate the reader to start reading from the first row again
		_, err = p.file.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		// Need to create a new reader because Go's underlying CSV reader uses a buffered input
		// so we need to initialise it with the file again
		nemReader = p.createNemReader(p.file)
	}

	data := make(UsageData)
	// Keep track of the current 200 record because it specifies the rules for the subsequent 300 blocks
	var current200 *NMIDataDetailsRecord
	// Keep track of the current 300 record because it needs to be adjusted by any subsequent 400 blocks
	var current300 *IntervalDataRecord
	// We can only handle certain kinds of 200 blocks (see IsValidReadingType). If we detect an
	// unsupported block, we skip it.
	skipping200 := false

	// Converts the current 300 interval record to an hourly reading. You would call this after you've
	// applied all the 400 records to the interval and are ready to finalise it.
	finalise300 := func() {
		if current300 != nil {
			readings := p.nem12IntervalToHourlyReadings(current200, current300)
			data[NMI(current200.NMI)][ReadingType(current200.NMISuffix)] =
				append(data[NMI(current200.NMI)][ReadingType(current200.NMISuffix)], readings...)
			current300 = nil
		}
	}

	records := make(chan []string)
	go p.getNem12Records(nemReader, records)

	for record := range records {
		if len(record) == 0 {
			continue
		}
		recordIndicator, err := strconv.Atoi(record[0])
		if err != nil {
			p.logger.Warn("Could not parse record indicator - moving to next line")
			continue
		}
		switch recordIndicator {
		case 200: // Data details
			// We've come back around to a 200 record so if we were skipping the previous block, we're done
			skipping200 = false
			finalise300()
			current200, err = p.parse200Record(record)
			if err != nil {
				return nil, err
			}
			if !IsValidReadingType(current200.NMISuffix) {
				p.logger.Error(fmt.Sprintf("200 record has unsupported suffix %v. Skipping block.", current200.NMISuffix))
				skipping200 = true
				continue
			}
			if data[NMI(current200.NMI)] == nil {
				data[NMI(current200.NMI)] = make(map[ReadingType][]HourlyReading)
			}
			if data[NMI(current200.NMI)][ReadingType(current200.NMISuffix)] == nil {
				data[NMI(current200.NMI)][ReadingType(current200.NMISuffix)] = make([]HourlyReading, 0)
			}
			p.logger.Debug("Parsed 200 record", slog.Any("record", current200))
		case 300: // Interval data
			if skipping200 {
				continue
			}
			finalise300()
			current300, err = p.parse300Record(record, current200)
			if err != nil {
				return nil, err
			}
			p.logger.Debug("Parsed 300 record", slog.Any("record", current300))
		case 400: // Interval event
			if skipping200 {
				continue
			}
			event, err := p.parse400Record(record, current200)
			if err != nil {
				return nil, err
			}
			p.adjustInterval(current300, event)
			p.logger.Debug("Parsed 400 record", slog.Any("record", event))
		case 500: // B2B details
			// This is a manual reading that provides the total recorded accumulated energy for a
			// Datastream retrieved from a meter’s register at the time of collection. It doesn't
			// really serve our purposes so we ignore it.
			continue
		case 900: // End of data
			finalise300()
			return data, nil
		default:
			p.logger.Warn(fmt.Sprintf("Unrecognised record indicator: %v", recordIndicator))
		}
	}
	p.logger.Warn("Missing 900 record")
	return data, nil
}

func (p *Parser) createNemReader(file *os.File) *csv.Reader {
	csvReader := csv.NewReader(file)
	// NEM12 files have variable fields per record, so we tell the CSV reader to not expect
	// any particular number
	csvReader.FieldsPerRecord = -1
	return csvReader
}

func (p *Parser) checkNem12Header(nemReader *csv.Reader) (bool, error) {
	record, err := nemReader.Read()
	if err != nil {
		return false, err
	}
	if record[0] != "100" {
		p.logger.Debug("No header record - assuming NEM12 format")
		return false, nil
	}
	if record[1] != "NEM12" {
		return true, errors.New("header record indicates this is not a NEM12 file")
	}
	return true, nil
}

func (p *Parser) getNem12Records(csvReader *csv.Reader, records chan<- []string) {
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			p.logger.Error(err.Error())
		}
		records <- record
	}
	close(records)
}

func (p *Parser) parse200Record(record []string) (*NMIDataDetailsRecord, error) {
	// Mandatory field
	intervalLength, err := strconv.Atoi(record[8])
	if err != nil {
		return nil, errors.New("interval length cannot be parsed")
	}

	return &NMIDataDetailsRecord{
		NMI:                     record[1],
		NMIConfiguration:        record[2],
		RegisterID:              record[3],
		NMISuffix:               record[4],
		MDMDataStreamIdentifier: record[5],
		MeterSerialNumber:       record[6],
		UOM:                     record[7],
		IntervalLength:          intervalLength,
		// We don't care about NextScheduledReadDate
	}, nil
}

func (p *Parser) parse300Record(record []string, currentDetails *NMIDataDetailsRecord) (*IntervalDataRecord, error) {
	// Mandatory field
	intervalDate, err := time.Parse("20060102", record[1])
	if err != nil {
		return nil, errors.New("interval date cannot be parsed")
	}

	// As per the spec, "The number of values provided must equal 1440 divided by the IntervalLength"
	// There are 2 other mandatory fields,
	numIntervalVals := 1440 / currentDetails.IntervalLength
	idxAfterIntervalVals := numIntervalVals + 2
	if len(record) < idxAfterIntervalVals {
		return nil, errors.New("not enough interval values")
	}

	vals := []IntervalValue{}
	for _, val := range record[2 : numIntervalVals+2] {
		valFlt, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, errors.New("interval value cannot be parsed")
		}
		vals = append(vals, IntervalValue{Value: valFlt})
	}

	var reasonCode *int
	if record[idxAfterIntervalVals+1] != "" {
		reasonCodeInt, err := strconv.Atoi(record[idxAfterIntervalVals+1])
		if err != nil {
			return nil, errors.New("reason code cannot be parsed")
		}
		reasonCode = &reasonCodeInt
	}

	return &IntervalDataRecord{
		IntervalDate:      intervalDate,
		IntervalValues:    vals,
		QualityMethod:     record[idxAfterIntervalVals],
		ReasonCode:        reasonCode,
		ReasonDescription: record[idxAfterIntervalVals+2],
		// We don't care about UpdateDateTime and MSATSLoadDateTime
	}, nil
}

func (p *Parser) parse400Record(record []string, currentDetails *NMIDataDetailsRecord) (*IntervalEventRecord, error) {
	// Mandatory field
	startInterval, err := strconv.Atoi(record[1])
	if err != nil {
		return nil, errors.New("start interval cannot be parsed")
	}
	endInterval, err := strconv.Atoi(record[2])
	if err != nil {
		return nil, errors.New("end interval cannot be parsed")
	}
	var reasonCode *int
	if record[4] != "" {
		reasonCodeInt, err := strconv.Atoi(record[4])
		if err != nil {
			return nil, errors.New("reason code cannot be parsed")
		}
		reasonCode = &reasonCodeInt
	}
	return &IntervalEventRecord{
		StartInterval:     startInterval,
		EndInterval:       endInterval,
		QualityMethod:     record[3],
		ReasonCode:        reasonCode,
		ReasonDescription: record[5],
	}, nil
}

// Applies a 400 interval event record to a given 300 interval data record
func (p *Parser) adjustInterval(currentInterval *IntervalDataRecord, event *IntervalEventRecord) {
	quality := QualityData{
		QualityMethod:     event.QualityMethod,
		ReasonCode:        event.ReasonCode,
		ReasonDescription: event.ReasonDescription,
	}
	// Intervals are 1-indexed and closed
	for i := event.StartInterval; i <= event.EndInterval; i++ {
		currentInterval.IntervalValues[i-1].Quality = &quality
	}
}

// Converts a 300 interval data record with corresponding 200 data detail into our own hourly reading structure
func (p *Parser) nem12IntervalToHourlyReadings(details *NMIDataDetailsRecord, interval *IntervalDataRecord) []HourlyReading {
	var err error
	readingsPerHour := 60 / details.IntervalLength
	hourlyReadings := make([]HourlyReading, 24)

	for hr := 0; hr < 24; hr++ {
		startTime := interval.IntervalDate.Add(time.Hour * time.Duration(hr))
		endTime := startTime.Add(time.Hour)
		energyTotal := 0.0
		qualityMethod := mapset.NewSet[string]()
		reasonCode := mapset.NewSet[int]()
		reasonDesc := mapset.NewSet[string]()
		// If the method on the interval record is V, we'll get the real methods from the inner values
		if interval.QualityMethod != "V" {
			qualityMethod.Add(interval.QualityMethod)
		}
		if interval.ReasonCode != nil {
			reasonCode.Add(*interval.ReasonCode)
		}
		if interval.ReasonDescription != "" {
			reasonDesc.Add(interval.ReasonDescription)
		}
		for i := 0; i < readingsPerHour; i++ {
			reading := interval.IntervalValues[(hr*readingsPerHour)+i]
			energyTotal = energyTotal + reading.Value
			if reading.Quality != nil {
				qualityMethod.Add(reading.Quality.QualityMethod)
				if reading.Quality.ReasonCode != nil {
					reasonCode.Add(*reading.Quality.ReasonCode)
				}
				if reading.Quality.ReasonDescription != "" {
					reasonDesc.Add(reading.Quality.ReasonDescription)
				}
			}
		}
		energyTotal, err = convertEnergy(energyTotal, details.UOM)
		if err != nil {
			p.logger.Error(err.Error())
			continue
		}
		hourlyReadings[hr] = HourlyReading{
			StartTime:         startTime,
			EndTime:           endTime,
			EnergyKWh:         float64(energyTotal),
			QualityMethod:     qualityMethod.ToSlice(),
			ReasonCode:        reasonCode.ToSlice(),
			ReasonDescription: reasonDesc.ToSlice(),
		}
	}

	return hourlyReadings
}

func convertEnergy(energy float64, uom string) (float64, error) {
	switch strings.ToLower(uom) {
	case "wh":
		return energy / 1000, nil
	case "kwh":
		return energy, nil
	case "mhh":
		return energy * 1000, nil
	default:
		return 0, fmt.Errorf("unsupported unit: %v", uom)
	}
}

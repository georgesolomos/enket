package nem12

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"golang.org/x/exp/slog"
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

func (p *Parser) Parse() {
	nemReader := p.createNemReader(p.file)

	hasHeader, err := p.checkNem12Header(nemReader)
	if err != nil {
		p.logger.Error(err.Error())
		os.Exit(1)
	}

	if !hasHeader {
		// Reset the file and recreate the reader to start reading from the first row again
		_, err = p.file.Seek(0, io.SeekStart)
		if err != nil {
			p.logger.Error(err.Error())
			os.Exit(1)
		}
		nemReader = p.createNemReader(p.file)
	}

	records := make(chan []string)
	go p.getNem12Records(nemReader, records)
	// Keep track of the current 200 record because it specifies the rules for the subsequent 300 blocks
	var currentDetails *NMIDataDetailsRecord
	// Keep track of the current 300 record because it needs to be adjusted for any subsequent 400 blocks
	var currentInterval *IntervalDataRecord
	// Keep track of whether we're currently adjusting a 300 record. Any number of 400 records can come
	// after a 300 to adjust different parts of it.
	adjustingInterval := false
	data := make(map[NMI]map[ReadingType]HourlyReading)
	for record := range records {
		if len(record) == 0 {
			continue
		}
		recordIndicator, err := strconv.Atoi(record[0])
		if err != nil {
			p.logger.Warn("Could not parse record indicator - moving to next line")
		}
		switch recordIndicator {
		case 200: // Data details
			adjustingInterval = false
			currentDetails, err = p.parse200Record(record)
			if err != nil {
				p.logger.Error(err.Error())
				return
			}
			hourlyReading := p.nem12IntervalToHourlyReading(currentInterval)
			intervalMap := map[ReadingType]HourlyReading{ReadingType(currentDetails.NMISuffix): hourlyReading}
			data[NMI(currentDetails.NMI)] = intervalMap
			p.logger.Debug("Parsed 200 record", slog.Any("record", currentDetails))
		case 300: // Interval data
			if adjustingInterval {
				data[NMI(currentDetails.NMI)][ReadingType(currentDetails.NMISuffix)] = p.nem12IntervalToHourlyReading(currentInterval)
				adjustingInterval = false
			}
			currentInterval, err := p.parse300Record(record, currentDetails)
			if err != nil {
				p.logger.Error(err.Error())
				return
			}
			p.logger.Debug("Parsed 300 record", slog.Any("record", currentInterval))
		case 400: // Interval event
			adjustingInterval = true
			event, err := p.parse400Record(record, currentDetails)
			if err != nil {
				p.logger.Error(err.Error())
				return
			}
			p.adjustInterval(currentInterval, event)
			p.logger.Debug("Parsed 400 record", slog.Any("record", event))
		case 500: // B2B details
			// This is a manual reading that provides the total recorded accumulated energy for a
			// Datastream retrieved from a meter’s register at the time of collection. It doesn't
			// really serve our purposes so we ignore it.
		case 900: // End of data
			return
		default:
			p.logger.Warn(fmt.Sprintf("Unrecognised record indicator: %v", recordIndicator))
		}
	}
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
			// This isn't great but we continue in case it's a one-off mistake
			p.logger.Warn("parse300Record: interval value cannot be parsed")
			valFlt = 0
		}
		vals = append(vals, IntervalValue{Value: valFlt})
	}

	reasonCode, err := strconv.Atoi(record[idxAfterIntervalVals+1])
	if err != nil {
		p.logger.Warn("parse300Record: reason code cannot be parsed")
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
	return &IntervalEventRecord{}, nil
}

func (p *Parser) adjustInterval(currentInterval *IntervalDataRecord, event *IntervalEventRecord) {
	quality := QualityData{
		QualityMethod:     event.QualityMethod,
		ReasonCode:        event.ReasonCode,
		ReasonDescription: event.ReasonDescription,
	}
	// Intervals are 1-indexed and closed
	for i := event.StartInterval; i <= event.EndInterval; i++ {
		currentInterval.IntervalValues[i+1].Quality = &quality
	}
}

func (p *Parser) nem12IntervalToHourlyReading(currentInterval *IntervalDataRecord) HourlyReading {
	return HourlyReading{}
}

func (p *Parser) checkNem12Header(nemReader *csv.Reader) (bool, error) {
	record, err := nemReader.Read()
	if err != nil {
		return false, err
	}
	if record[0] != "100" {
		p.logger.Info("No header record - assuming NEM12 format")
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

func (p *Parser) createNemReader(file *os.File) *csv.Reader {
	csvReader := csv.NewReader(file)
	// NEM12 files have variable fields per record, so we tell the CSV reader to not expect
	// any particular number
	csvReader.FieldsPerRecord = -1
	return csvReader
}

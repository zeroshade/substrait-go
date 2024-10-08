package literal

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/substrait-io/substrait-go/expr"
	"github.com/substrait-io/substrait-go/proto"
	"github.com/substrait-io/substrait-go/types"
)

func NewBool(value bool) (expr.Literal, error) {
	return expr.NewPrimitiveLiteral[bool](value, false), nil
}

func NewInt8(value int8) (expr.Literal, error) {
	return expr.NewPrimitiveLiteral[int8](value, false), nil
}

func NewInt16(value int16) (expr.Literal, error) {
	return expr.NewPrimitiveLiteral[int16](value, false), nil
}

func NewInt32(value int32) (expr.Literal, error) {
	return expr.NewPrimitiveLiteral[int32](value, false), nil
}

func NewInt64(value int64) (expr.Literal, error) {
	return expr.NewPrimitiveLiteral[int64](value, false), nil
}

func NewFloat32(value float32) (expr.Literal, error) {
	return expr.NewPrimitiveLiteral[float32](value, false), nil
}

func NewFloat64(value float64) (expr.Literal, error) {
	return expr.NewPrimitiveLiteral[float64](value, false), nil
}

func NewString(value string) (expr.Literal, error) {
	return expr.NewPrimitiveLiteral[string](value, false), nil
}

func NewDate(days int) (expr.Literal, error) {
	return expr.NewLiteral[types.Date](types.Date(days), false)
}

// NewTime creates a new Time literal from the given hours, minutes, seconds and microseconds.
// The total microseconds should be in the range [0, 86400_000_000) to represent a valid time within a day.
func NewTime(hours, minutes, seconds, microseconds int32) (expr.Literal, error) {
	duration := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second + time.Duration(microseconds)*time.Microsecond
	micros := duration.Microseconds()
	if micros < 0 || micros >= (24*time.Hour).Microseconds() {
		return nil, fmt.Errorf("invalid time value %d:%d:%d.%d", hours, minutes, seconds, microseconds)
	}
	return expr.NewLiteral[types.Time](types.Time(duration.Microseconds()), false)
}

// NewTimeFromMicros creates a new Time literal from the given microseconds.
func NewTimeFromMicros(micros int64) (expr.Literal, error) {
	if micros < 0 || micros >= (24*time.Hour).Microseconds() {
		return nil, fmt.Errorf("invalid time value %d", micros)
	}
	return expr.NewLiteral[types.Time](types.Time(micros), false)
}

// NewTimestamp creates a new Timestamp literal from a time.Time timestamp value.
// This uses the number of microseconds elapsed since January 1, 1970 00:00:00 UTC
func NewTimestamp(timestamp time.Time) (expr.Literal, error) {
	return expr.NewLiteral[types.Timestamp](types.Timestamp(timestamp.UnixMicro()), false)
}

func NewTimestampFromMicros(micros int64) (expr.Literal, error) {
	return expr.NewLiteral[types.Timestamp](types.Timestamp(micros), false)
}

// NewTimestampTZ creates a new TimestampTz literal from a time.Time timestamp value.
// This uses the number of microseconds elapsed since January 1, 1970 00:00:00 UTC
func NewTimestampTZ(timestamp time.Time) (expr.Literal, error) {
	return expr.NewLiteral[types.TimestampTz](types.TimestampTz(timestamp.UnixMicro()), false)
}

func NewTimestampTZFromMicros(micros int64) (expr.Literal, error) {
	return expr.NewLiteral[types.TimestampTz](types.TimestampTz(micros), false)
}

func NewIntervalYearsToMonth(years, months int32) (expr.Literal, error) {
	return expr.NewLiteral[*types.IntervalYearToMonth](&types.IntervalYearToMonth{Years: years, Months: months}, false)
}

func NewIntervalDaysToSecond(days, seconds int32, micros int64) (expr.Literal, error) {
	return expr.NewLiteral[*types.IntervalDayToSecond](&types.IntervalDayToSecond{
		Days:    days,
		Seconds: seconds,
		PrecisionMode: &proto.Expression_Literal_IntervalDayToSecond_Precision{
			Precision: int32(types.PrecisionMicroSeconds),
		},
		Subseconds: micros,
	}, false)
}

func NewUUID(guid uuid.UUID) (expr.Literal, error) {
	bytes, err := guid.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return expr.NewLiteral[types.UUID](bytes, false)
}

func NewUUIDFromBytes(value []byte) (expr.Literal, error) {
	return expr.NewLiteral[types.UUID](value, false)
}

func NewFixedChar(value string) (expr.Literal, error) {
	return expr.NewLiteral[types.FixedChar](types.FixedChar(value), false)
}

func NewFixedBinary(value []byte) (expr.Literal, error) {
	return expr.NewLiteral[types.FixedBinary](value, false)
}

func NewVarChar(value string) (expr.Literal, error) {
	return expr.NewLiteral[*types.VarChar](&types.VarChar{Value: value, Length: uint32(len(value))}, false)
}

// NewDecimalFromTwosComplement create a Decimal literal from twosComplement.
// twosComplement is a little-endian twos-complement integer representation of complete value
func NewDecimalFromTwosComplement(twosComplement []byte, precision, scale int32) (expr.Literal, error) {
	if len(twosComplement) != 16 {
		return nil, fmt.Errorf("twosComplement must be 16 bytes")
	}
	if precision < 1 || precision > 38 {
		return nil, fmt.Errorf("precision must be in range [1, 38]")
	}
	if scale < 0 || scale > precision {
		return nil, fmt.Errorf("scale must be in range [0, precision]")
	}
	return expr.NewLiteral[*types.Decimal](&types.Decimal{Value: twosComplement, Precision: precision, Scale: scale}, false)

}

// NewDecimalFromString create a Decimal literal from decimal value string
func NewDecimalFromString(value string) (expr.Literal, error) {
	v, precision, scale, err := decimalStringToBytes(value)
	if err != nil {
		return nil, err
	}
	return expr.NewLiteral[*types.Decimal](&types.Decimal{Value: v[:16], Precision: precision, Scale: scale}, false)
}

// NewPrecisionTimestampFromTime creates a new PrecisionTimestamp literal from a time.Time timestamp value with given precision.
func NewPrecisionTimestampFromTime(precision types.TimePrecision, tm time.Time) (expr.Literal, error) {
	return NewPrecisionTimestamp(precision, getTimeValueByPrecision(tm, precision))
}

// NewPrecisionTimestamp creates a new PrecisionTimestamp literal with given precision and value.
func NewPrecisionTimestamp(precision types.TimePrecision, value int64) (expr.Literal, error) {
	return expr.NewLiteral[*types.PrecisionTimestamp](&types.PrecisionTimestamp{
		PrecisionTimestamp: &proto.Expression_Literal_PrecisionTimestamp{
			Precision: int32(precision),
			Value:     value,
		},
	}, false)
}

// NewPrecisionTimestampTzFromTime creates a new PrecisionTimestampTz literal from a time.Time timestamp value with given precision.
func NewPrecisionTimestampTzFromTime(precision types.TimePrecision, tm time.Time) (expr.Literal, error) {
	return NewPrecisionTimestampTz(precision, getTimeValueByPrecision(tm, precision))
}

// NewPrecisionTimestampTz creates a new PrecisionTimestampTz literal with given precision and value.
func NewPrecisionTimestampTz(precision types.TimePrecision, value int64) (expr.Literal, error) {
	return expr.NewLiteral[*types.PrecisionTimestampTz](&types.PrecisionTimestampTz{
		PrecisionTimestampTz: &proto.Expression_Literal_PrecisionTimestamp{
			Precision: int32(precision),
			Value:     value,
		},
	}, false)
}

func getTimeValueByPrecision(tm time.Time, precision types.TimePrecision) int64 {
	switch precision {
	case types.PrecisionSeconds:
		return tm.Unix()
	case types.PrecisionDeciSeconds:
		return tm.UnixMilli() / 100
	case types.PrecisionCentiSeconds:
		return tm.UnixMilli() / 10
	case types.PrecisionMilliSeconds:
		return tm.UnixMilli()
	case types.PrecisionEMinus4Seconds:
		return tm.UnixMicro() / 100
	case types.PrecisionEMinus5Seconds:
		return tm.UnixMicro() / 10
	case types.PrecisionMicroSeconds:
		return tm.UnixMicro()
	case types.PrecisionEMinus7Seconds:
		return tm.UnixNano() / 100
	case types.PrecisionEMinus8Seconds:
		return tm.UnixNano() / 10
	case types.PrecisionNanoSeconds:
		return tm.UnixNano()
	default:
		panic(fmt.Sprintf("unknown TimePrecision %v", precision))
	}
}

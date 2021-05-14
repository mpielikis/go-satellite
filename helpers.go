package satellite

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Constants
const TWOPI float64 = math.Pi * 2.0
const DEG2RAD float64 = math.Pi / 180.0
const RAD2DEG float64 = 180.0 / math.Pi
const XPDOTP float64 = 1440.0 / (2.0 * math.Pi)

// Holds latitude and Longitude in either degrees or radians
type LatLong struct {
	Latitude, Longitude float64
}

// Holds latitude and Longitude in either degrees or radians
type LatLongAlt struct {
	LatLong    LatLong
	AltitudeKm float64
}

// Holds X, Y, Z position
type Vector3 struct {
	X, Y, Z float64
}

// Holds an azimuth, elevation and range
type LookAngles struct {
	Az, El, Rg float64
}

type JDay struct {
	Day, Fraction float64
}

// Parses a two line element dataset into a Satellite struct
func ParseTLE(line1, line2, gravconst string) (sat Satellite, err error) {

	if len(line1) != 69 {
		return sat, fmt.Errorf("Line1 length should be 69 but was %d", len(line1))
	}

	if len(line2) != 69 {
		return sat, fmt.Errorf("Line2 length should be 69 but was %d", len(line2))
	}

	sat.Line1 = line1
	sat.Line2 = line2

	sat.Error = 0
	sat.Whichconst, err = getGravConst(gravconst)
	if err != nil {
		err = fmt.Errorf("Error on getting gravconst: %v", err)
		return
	}

	// LINE 1 BEGIN
	sat.satnum, err = parseInt(strings.TrimSpace(line1[2:7]))
	if err != nil {
		err = fmt.Errorf("Error on parsing line1[2:7]: %v", err)
		return
	}

	sat.epochyr, err = parseInt(line1[18:20])
	if err != nil {
		err = fmt.Errorf("Error on parsing line1[18:20]: %v", err)
		return
	}
	sat.epochdays, err = parseFloat(line1[20:32])
	if err != nil {
		err = fmt.Errorf("Error on parsing line1[20:32]: %v", err)
		return
	}

	// These three can be negative / positive
	sat.ndot, err = parseFloat(strings.Replace(line1[33:43], " ", "", 2))
	if err != nil {
		err = fmt.Errorf("Error on parsing line1[33:43]: %v", err)
		return
	}
	sat.nddot, err = parseFloat(strings.Replace(line1[44:45]+"."+line1[45:50]+"e"+line1[50:52], " ", "", 2))
	if err != nil {
		err = fmt.Errorf("Error on parsing line1[44:52]: %v", err)
		return
	}
	sat.bstar, err = parseFloat(strings.Replace(line1[53:54]+"."+line1[54:59]+"e"+line1[59:61], " ", "", 2))
	if err != nil {
		err = fmt.Errorf("Error on parsing line1[53:61]: %v", err)
		return
	}
	// LINE 1 END

	// LINE 2 BEGIN
	sat.inclo, err = parseFloat(strings.Replace(line2[8:16], " ", "", 2))
	if err != nil {
		err = fmt.Errorf("Error on parsing line2[8:16]: %v", err)
		return
	}
	sat.nodeo, err = parseFloat(strings.Replace(line2[17:25], " ", "", 2))
	if err != nil {
		err = fmt.Errorf("Error on parsing line2[17:25]: %v", err)
		return
	}
	sat.ecco, err = parseFloat("." + line2[26:33])
	if err != nil {
		err = fmt.Errorf("Error on parsing line2[26:33]: %v", err)
		return
	}
	sat.argpo, err = parseFloat(strings.Replace(line2[34:42], " ", "", 2))
	if err != nil {
		err = fmt.Errorf("Error on parsing line2[34:42]: %v", err)
		return
	}
	sat.mo, err = parseFloat(strings.Replace(line2[43:51], " ", "", 2))
	if err != nil {
		err = fmt.Errorf("Error on parsing line2[43:51]: %v", err)
		return
	}
	sat.no, err = parseFloat(strings.Replace(line2[52:63], " ", "", 2))
	if err != nil {
		err = fmt.Errorf("Error on parsing line2[52:63]: %v", err)
		return
	}
	// LINE 2 END
	return
}

// Converts a two line element data set into a Satellite struct and runs sgp4init
func NewSatFromTLE(line1, line2 string, gravconst string) (Satellite, error) {
	//sat := Satellite{Line1: line1, Line2: line2}
	sat, err := ParseTLE(line1, line2, gravconst)

	if err != nil {
		return sat, err
	}

	opsmode := "i"

	sat.no = sat.no / XPDOTP
	sat.ndot = sat.ndot / (XPDOTP * 1440.0)
	sat.nddot = sat.nddot / (XPDOTP * 1440.0 * 1440)

	sat.inclo = sat.inclo * DEG2RAD
	sat.nodeo = sat.nodeo * DEG2RAD
	sat.argpo = sat.argpo * DEG2RAD
	sat.mo = sat.mo * DEG2RAD

	var year int64 = 0
	if sat.epochyr < 57 {
		year = sat.epochyr + 2000
	} else {
		year = sat.epochyr + 1900
	}

	mon, day, hr, min, sec := days2mdhms(year, sat.epochdays)

	sat.jdsatepoch = NewJDay(int(year), int(mon), int(day), int(hr), int(min), sec)

	sgp4init(&opsmode, sat.jdsatepoch.Subtract(2433281.5), &sat)

	return sat, nil
}

func NewLatLongAlt(latitudeDeg, longitudeDeg, altitudeKm float64) LatLongAlt {
	return LatLongAlt{
		LatLong: LatLong{
			Latitude:  DEG2RAD * latitudeDeg,
			Longitude: DEG2RAD * longitudeDeg},
		AltitudeKm: altitudeKm,
	}
}

func (jd JDay) Subtract(time float64) float64 {
	return jd.Day + jd.Fraction - time
}

func (jd JDay) SubtractDay(j JDay) float64 {
	return (jd.Day-j.Day)*1440 + (jd.Fraction-j.Fraction)*1440
}

func (jd JDay) Single() float64 {
	return jd.Day + jd.Fraction
}

// Parses a string into a float64 value.
func parseFloat(strIn string) (ret float64, err error) {
	strIn = strings.Replace(strIn, " ", "0", -1)
	return strconv.ParseFloat(strIn, 64)
}

// Parses a string into a int64 value.
func parseInt(strIn string) (int64, error) {
	return strconv.ParseInt(strIn, 10, 0)
}

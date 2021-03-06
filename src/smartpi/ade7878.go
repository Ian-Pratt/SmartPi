/*
    Copyright (C) Jens Ramhorst
	  This file is part of SmartPi.
    SmartPi is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.
    SmartPi is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.
    You should have received a copy of the GNU General Public License
    along with SmartPi.  If not, see <http://www.gnu.org/licenses/>.
    Diese Datei ist Teil von SmartPi.
    SmartPi ist Freie Software: Sie können es unter den Bedingungen
    der GNU General Public License, wie von der Free Software Foundation,
    Version 3 der Lizenz oder (nach Ihrer Wahl) jeder späteren
    veröffentlichten Version, weiterverbreiten und/oder modifizieren.
    SmartPi wird in der Hoffnung, dass es nützlich sein wird, aber
    OHNE JEDE GEWÄHRLEISTUNG, bereitgestellt; sogar ohne die implizite
    Gewährleistung der MARKTFÄHIGKEIT oder EIGNUNG FÜR EINEN BESTIMMTEN ZWECK.
    Siehe die GNU General Public License für weitere Details.
    Sie sollten eine Kopie der GNU General Public License zusammen mit diesem
    Programm erhalten haben. Wenn nicht, siehe <http://www.gnu.org/licenses/>.
*/

package smartpi

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"os"
	rpi "github.com/nathan-osman/go-rpigpio"
	"golang.org/x/exp/io/i2c"

	log "github.com/sirupsen/logrus"
)

const (
	ADE7878_ADDR int     = 0x38
	SAMPLES      int     = 100
	ade7878Clock float64 = 256000
	halfCircle   float64 = math.Pi / 180.0
)

type Phase uint

const (
	_ = iota
	PhaseA
	PhaseB
	PhaseC
	PhaseN
)

func (p Phase) String() string {
	switch p {
	case PhaseA:
		return "A"
	case PhaseB:
		return "B"
	case PhaseC:
		return "C"
	case PhaseN:
		return "N"
	}
	panic("Unreachable")
}

func (p Phase) PhaseNumber() string {
	switch p {
	case PhaseA:
		return "1"
	case PhaseB:
		return "2"
	case PhaseC:
		return "3"
	case PhaseN:
		return "4"
	}
	panic("Unreachable")
}

var MainPhases = []Phase{PhaseA, PhaseB, PhaseC}

type Readings map[Phase]float64

type ADE7878Readout struct {
	Current       Readings
	Voltage       Readings
	ActiveWatts   Readings
	CosPhi        Readings
	Frequency     Readings
	ApparentPower Readings
	ReactivePower Readings
	PowerFactor   Readings
	ActiveEnergy  Readings
}

type CTFactors struct {
	CurrentResistor, CurrentClampFactor, OffsetCurrent, OffsetVoltage, PowerCorrectionFactor float64
}

var (
	CTTypes = map[string]CTFactors{
		"YHDC_SCT013": CTFactors{
			//CurrentResistor:       7.07107,
			CurrentResistor:       7.47,
			CurrentClampFactor:    0.05,
			OffsetCurrent:         1.049084906,
			OffsetVoltage:         1.0,
			PowerCorrectionFactor: 0.019413,
		},
		"X/1A": CTFactors{
			CurrentResistor:       0.33,
			CurrentClampFactor:    1.0,
			OffsetCurrent:         1.010725941,
			OffsetVoltage:         1.0,
			PowerCorrectionFactor: 0.043861,
		},
	}
)

var (
	rms_factor_current float64
)

// Fetch a number of bytes from the device and convert it to an int.
func DeviceFetchInt(d *i2c.Device, l int, cmd []byte) int64 {
	startTime := time.Now()
	err := d.Write(cmd)
	if err != nil {
		panic(err)
	}
	data := make([]byte, l)
	err = d.Read(data)
	if err != nil {
		panic(err)
	}
	var result int64
	switch l {
	case 8:
		result = int64(binary.BigEndian.Uint64(data))
	case 4:
		result = int64(int32(binary.BigEndian.Uint32(data)))
	case 2:
		result = int64(int16(binary.BigEndian.Uint16(data)))
	default:
		panic(fmt.Errorf("Invalid byte length for int conversion %d", l))
	}
	log.Debugf("DeviceFetchInt: %s cmd: %x data: %x result: %d", time.Since(startTime), cmd, data, result)
	return result
}

func resetADE7878(d *i2c.Device) {
	//println("RESET")
	p, err := rpi.OpenPin(4, rpi.OUT)
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Write(rpi.HIGH)
	time.Sleep(time.Millisecond*200)
	p.Write(rpi.LOW)
        time.Sleep(time.Millisecond*200)

        for {
            time.Sleep(time.Millisecond*1)
            a := DeviceFetchInt(d, 4, ADE7878REG["STATUS1"])
            //fmt.Printf("%08x\n",a)
            if a & 0x8000 != 0 {
                //println("write status")
                WriteRegister(d, "STATUS1", 0xff, 0xff, 0xff, 0xff)
                break
            }
        }

}

func initPiForADE7878() {
	/*
	   p, err := rpi.OpenPin(4, rpi.OUT)
	   if err != nil {
	       panic(err)
	   }
	   defer p.Close()
	   p.Write(rpi.HIGH)*/
}

func WriteRegister(d *i2c.Device, register string, data ...byte) (err error) {
	return d.Write(append(ADE7878REG[register], data...))
}


func WriteRegisterV(d *i2c.Device, register string, data ...byte) (err error) {
        r := d.Write(append(ADE7878REG[register], data...))
        w := int64(binary.BigEndian.Uint32(data)) & 0x0fffffff
	a := DeviceFetchInt(d, 4, ADE7878REG[register])
        //println(register, "r=",r, data, "w=", w, "a=",a)
	//fmt.Printf("%s %08x %08x\n",register,w,a) 
        if w != a { 
		panic(r)
        }
        return r
}

func ReadRegisterX(d *i2c.Device, register string) int64 {
        a := DeviceFetchInt(d, 4, ADE7878REG[register])
        //println(register, "a=",a)
        //fmt.Printf("Read %s %08x\n",register,a)
        return a
}

var fi *os.File
var nextfileTime time.Time

func newfile( n time.Time ) () {
	if fi != nil {
		fi.Close()
	}
	s:= fmt.Sprintf("3phase-%s.log",n.Format("2006-01-02T15:04:05-07:00"))
	t, err := os.Create(s)
	if err != nil {
		panic( err )
	}
	fi = t
	
	y, m, d := n.Date()
    nextfileTime = time.Date(y, m, d+1, 0, 0, 0, 0, n.Location())
}


func InitADE7878(c *Config) (*i2c.Device, error) {

	d, err := i2c.Open(&i2c.Devfs{Dev: c.I2CDevice}, ADE7878_ADDR)
	if err != nil {
		panic(err)
	}

        resetADE7878(d)

	// 0xEC01 (CONFIG2-REGISTER)
	// 00000010 --> I2C-Lock
	//err = d.Write(append(ADE7878REG["CONFIG2"], 0x02))
	err = WriteRegister(d, "CONFIG2", 0x02)
	if err != nil {
		panic(err)
	}

	// 0xE1
	err = d.Write([]byte{0xEC})
	if err != nil {
		panic(err)
	}

	// Read i2cLock
	i2cLock := make([]byte, 1)
	err = d.Read(i2cLock)
	if err != nil {
		panic(err)
	}

        // it appears the DSP must be running, otherwise CIRMSOS/NIRMSOS appear to be OK. After RUN they read out 0
        // 0xE228 (RUN-Register)
        err = WriteRegister(d, "RUN", 0x00, 0x01)
        if err != nil {
                panic(err)
        }

	// 0xE7FE writeprotection
	err = d.Write([]byte{0xE7, 0xFE, 0xAD})
	if err != nil {
		panic(err)
	}

	// 0xE7E3 writeprotection OFF
	err = d.Write([]byte{0xE7, 0xE3, 0x00})
	if err != nil {
		panic(err)
	}

	// // 0x43B6 (HPFDIS-REGISTER)
	// err = d.Write(append(ADE7878REG["HPFDIS"], 0x00, 0x00, 0x00, 0x00})
	// if err != nil {
	//     panic(err)
	// }

	// 0x43B5 (DICOEFF-REGISTER)
	err = WriteRegister(d, "DICOEFF", 0xFF, 0xFF, 0x80, 0x00)
	if err != nil {
		panic(err)
	}

        // HACK IAP TODO - we are not using Integrator. We only have A channel voltage connected so feed to all channels
        // 16 bit regsiter 11:10 = 10 ; 13:12 = 01    1800   
        err = WriteRegister(d, "CONFIG", 0x18, 0x00 )
        if err != nil {
                panic (err)
        }


	if c.Integrator == true {

		// 0xE618 (CONFIG-REGISTER)
		err = WriteRegister(d, "CONFIG", 0x00, 0x01)
		if err != nil {
			panic(err)
		}

	}

	// Set the right power frequency to the COMPMODE-REGISTER.
	// 0xE60E (COMPMODE-REGISTER)
	if c.PowerFrequency == 60 {
		// 0x41FF 60Hz
		err = WriteRegister(d, "COMPMODE", 0x41, 0xFF)
	} else {
		// 0x01FF 50Hz
		err = WriteRegister(d, "COMPMODE", 0x01, 0xFF)
	}
	if err != nil {
		panic(err)
	}

        // WTHR=394616854 ?????
	//0x43AB (WTHR1-REGISTER)
	err = WriteRegisterV(d, "WTHR1", 0x00, 0x00, 0x00, 0x17)
	if err != nil {
		panic(err)
	}

	//0x43AC (WTHR0-REGISTER)
	err = WriteRegisterV(d, "WTHR0", 0x00, 0x85, 0x60, 0x16)
	if err != nil {
		panic(err)
	}

	// // 0x43AD (VARTHR1-REGISTER)
	// err = d.Write(append(ADE7878REG["VARTHR1"], 0x17, 0x85, 0x60, 0x16))
	// if err != nil {
	//     panic(err)
	// }
	//
	// // 0x43AE (VARTHR0-REGISTER)
	// err = d.Write(append(ADE7878REG["VARTHR0"], 0x17, 0x85, 0x60, 0x16))
	// if err != nil {
	//     panic(err)
	// }
	//
	// // 0x43A9 (VATHR1-REGISTER)
	// err = d.Write(append(ADE7878REG["VATHR1"], 0x17, 0x85, 0x60, 0x16))
	// if err != nil {
	//     panic(err)
	// }
	//
	// // 0x43AA (VATHR0-REGISTER)
	// err = d.Write(append(ADE7878REG["VATHR0"], 0x17, 0x85, 0x60, 0x16))
	// if err != nil {
	//     panic(err)
	// }

	// 0x43B3 (VLEVEL-REGISTER)
	err = WriteRegisterV(d, "VLEVEL", 0x00, 0x0C, 0xEC, 0x85)
	if err != nil {
		panic(err)
	}

        //	time.Sleep(875 * time.Millisecond)

	// // 0x4381 (AVGAIN-REGISTER)
	// outcome := DeviceFetchInt(d, 4, ADE7878REG["AVGAIN"])
	// fmt.Printf("AVGAIN-REGISTER VORHER%g   %x %x %x %x \n", outcome, data[0], data[1], data[2], data[3])

	// 0x4381 (AVGAIN-REGISTER)
	err = WriteRegisterV(d, "AVGAIN", 0xFF, 0xFC, 0x1C, 0xC2)
	if err != nil {
		panic(err)
	}

	// 0x4383 (BVGAIN-REGISTER)
	// err = WriteRegister(d, "BVGAIN", 0xFF, 0xFB, 0xCA, 0x60)
	err = WriteRegisterV(d, "BVGAIN", 0xFF, 0xFC, 0x1C, 0xC2)
	if err != nil {
		panic(err)
	}

	// 0x4385 (CVGAIN-REGISTER)
	//err = WriteRegister(d, "CVGAIN", 0xFF, 0xFC, 0x12, 0xDE)
	err = WriteRegisterV(d, "CVGAIN", 0xFF, 0xFC, 0x1C, 0xC2)
	if err != nil {
		panic(err)
	}

	err = WriteRegisterV(d, "AIRMSOS", 0x0F, 0xFD, 0x40, 0xE0)
	if err != nil {
		panic(err)
	}

	err = WriteRegisterV(d, "BIRMSOS", 0x0F, 0xFD, 0x40, 0xE0)
	if err != nil {
		panic(err)
	}

	err = WriteRegisterV(d, "CIRMSOS", 0x0F, 0xFD, 0x40, 0xE0)
	if err != nil {
		panic(err)
	}

	err = WriteRegisterV(d, "NIRMSOS", 0x0F, 0xFD, 0x40, 0xE0)
	if err != nil {
		panic(err)
	}

        // freq measurement, set to Voltage A phase`
        err = WriteRegister(d, "MMODE", 0x1c )
        if err != nil {
                panic(err)
        }

	// Line cycle mode
	// 0xE702 LCYCMODE count 200 on V1 Voltage A phase -- others wobble as noy connected
	err = WriteRegister(d, "LCYCMODE", 0x00, 0x00, 0x00, 0x0f)  // 0x0f
	if err != nil {
		panic(err)
	}

//a := ReadRegisterX(d, "LCYCMODE")
//fmt.Printf("LCYCMODE=%08x\n",a)

	// Line cycle mode count c8=200 (2sec)  / 0x64 is 100 (1sec) 
	// 0xE60C LINECYC
	err = WriteRegister(d, "LINECYC", 0x00, 0x00, 0x00, 0x64 )  // 0xC8)
	if err != nil {
		panic(err)
	}

//a = ReadRegisterX(d, "LINECYC")
//fmt.Printf("LINECYC=%08x\n",a)

        // data sheet suggests writting the last register 3 times
/*
        WriteRegister(d, "LINECYC", 0xC8)
        WriteRegister(d, "LINECYC", 0xC8)
        WriteRegister(d, "LINECYC", 0xC8)
*/

/*
ReadRegisterX(d, "AIGAIN")
ReadRegisterX(d, "BIGAIN")
ReadRegisterX(d, "CIGAIN")
ReadRegisterX(d, "NIGAIN")
ReadRegisterX(d, "AVGAIN") 
ReadRegisterX(d, "BVGAIN")
ReadRegisterX(d, "CVGAIN")
ReadRegisterX(d, "NVGAIN")  
ReadRegisterX(d, "AIRMSOS")
ReadRegisterX(d, "BIRMSOS")
ReadRegisterX(d, "CIRMSOS")
ReadRegisterX(d, "NIRMSOS")
*/

	// 0xE7FE writeprotection
	err = d.Write([]byte{0xE7, 0xFE, 0xAD})
	if err != nil {
		panic(err)
	}

	// 0xE7E3 writeprotection
	err = d.Write([]byte{0xE7, 0xE3, 0x80})
	if err != nil {
		panic(err)
	}

/*
ReadRegisterX(d, "AIGAIN")
ReadRegisterX(d, "BIGAIN")
ReadRegisterX(d, "CIGAIN")
ReadRegisterX(d, "NIGAIN")
ReadRegisterX(d, "NVGAIN")
ReadRegisterX(d, "AIRMSOS")
ReadRegisterX(d, "BIRMSOS")
ReadRegisterX(d, "CIRMSOS")
ReadRegisterX(d, "NIRMSOS")
*/

/* already running 
        // it appears the DSP must be running, otherwise CIRMSOS/NIRMSOS appear to be OK. After RUN they read out 0
	// 0xE228 (RUN-Register)
	err = WriteRegister(d, "RUN", 0x00, 0x01)
	if err != nil {
		panic(err)
	}
*/

/*
ReadRegisterX(d, "AIGAIN")
ReadRegisterX(d, "BIGAIN")
ReadRegisterX(d, "CIGAIN")
ReadRegisterX(d, "NIGAIN")
ReadRegisterX(d, "AVGAIN")
ReadRegisterX(d, "BVGAIN")
ReadRegisterX(d, "CVGAIN")
ReadRegisterX(d, "NVGAIN")
ReadRegisterX(d, "AIRMSOS")
ReadRegisterX(d, "BIRMSOS")
ReadRegisterX(d, "CIRMSOS")
ReadRegisterX(d, "NIRMSOS")
*/

newfile(time.Now())
WriteRegister(d,"STATUS0", 0xff, 0xff, 0xff, 0xff)
t1 := int64(0)
tx := 0.0
spin := 0
         for {
//break
            a := ReadRegisterX(d,"STATUS0")
            //fmt.Printf("--%08x\n",a)
            if a & (1<<5) != 0 {
                currentTime := time.Now()
                outcome := float64(DeviceFetchInt(d, 2, []byte{0xE6, 0x07}))
                frequency := ade7878Clock / (outcome + 1)

/*
CurrentResistor:       7.47,                                                                                            CurrentClampFactor:    0.05,                                                                                            OffsetCurrent:         1.049084906,
OffsetVoltage:         1.0,                                                                                             PowerCorrectionFactor: 0.019413 
ccf = 1.0 / (float64(c.CTTypePrimaryCurrent[phase]) / 100.0)
current = ((((outcome * 0.3535) / rmsFactor) / cr) / ccf) * 100.0 * oc * c.CalibrationfactorI[phase]
*/  
h := ReadRegisterX(d, "AIRMS")
//fmt.Printf("%08x\n",h)
c := (((((float64(h))*0.3535)/4191910.0) / 7.47)     / 0.050) * 100.0 * 1.049084906 * 1.0
i1 := c
h = ReadRegisterX(d, "BIRMS")
c = (((((float64(h))*0.3535)/4191910.0) / 7.47)     / 0.050) * 100.0 * 1.049084906 * 1.0
i2 := c
h = ReadRegisterX(d, "CIRMS")
c = (((((float64(h))*0.3535)/4191910.0) / 7.47)     / 0.050) * 100.0 * 1.049084906 * 1.0
i3 := c
h = ReadRegisterX(d, "NIRMS")
c = (((((float64(h))*0.3535)/4191910.0) / 7.47)     / 0.050) * 100.0 * 1.049084906 * 1.0
i4 := c

h = ReadRegisterX(d, "AVRMS")
v1 := (float64(h) / 1e+4) 

h = ReadRegisterX(d, "AWATT")
p1 := ((float64)(h)) * 0.019413 
//fmt.Printf("%10x\n", h)
h = ReadRegisterX(d, "BWATT")
p2 := ((float64)(h)) * 0.019413
h = ReadRegisterX(d, "CWATT")
p3 := ((float64)(h)) * 0.019413

e1 := ReadRegisterX(d, "AWATTHR")
e2 := ReadRegisterX(d, "BWATTHR")
e3 := ReadRegisterX(d, "CWATTHR")
if e1 < 0 {
    e1 = e1 * -1.0 
}
if e2 < 0 {
    e2 = e2 * -1.0
}
if e3 < 0 {
    e3 = e3 * -1.0
}
t1 = t1 + e1 + e2 + e3
tx = tx + ((i1+i2+i3)*v1)
//fmt.Printf("%d %f\n",h, e1)
//XXX

if currentTime.After(nextfileTime) {
	newfile(currentTime)
}

                st := fmt.Sprintf("%s f=%6.3f I1=%6.3f I2=%6.3f I3=%6.3f I4=%6.3f V1=%7.3f P1=%6.0f P2=%6.0f P3=%6.0f E1=%3d E2=%3d E3=%3d T1=%5d PX=%4.0f TX=%6.0f s=%d\n",currentTime.Format("2006-01-02T15:04:05.000-07:00"), frequency, i1, i2, i3, i4, v1, p1, p2, p3, e1, e2, e3, t1, (i1+i2+i3)*v1, tx/3600, spin)
				//println( st )
				fi.WriteString( st )
				//fmt.Printf("%s\n",currentTime.String())
//time.Sleep(time.Millisecond*950)

//fmt.Printf("S %s\n",time.Now())
nextTime := currentTime.Add(time.Millisecond*989)
time.Sleep(time.Until(nextTime))
//fmt.Printf("F %s %s\n", time.Now(), nextTime)
			   WriteRegister(d, "STATUS0", 0x00, 0x00, 0x00, 0x20)
			   spin = 0
			}
		spin = spin + 1	
        }



	return d, nil
}

func ReadCurrent(d *i2c.Device, c *Config, phase Phase) (current float64) {
	command := make([]byte, 2)
	switch phase {
	case PhaseA:
		command = ADE7878REG["AIRMS"] // 0x43C0 (AIRMS; Current rms an A)
	case PhaseB:
		command = ADE7878REG["BIRMS"] // 0x43C2 (AIRMS; Current rms an B)
	case PhaseC:
		command = ADE7878REG["CIRMS"] // 0x43C4 (AIRMS; Current rms an C)
	case PhaseN:
		command = ADE7878REG["NIRMS"] // 0x43C6 (AIRMS; Current rms an N)
	default:
		panic(fmt.Errorf("Invalid phase %q", phase))
	}

	var rmsFactor float64
	rmsFactor = 4191910.0 // 50Hz

	if c.MeasureCurrent[phase] {
		outcome := float64(DeviceFetchInt(d, 4, command))
		cr := CTTypes[c.CTType[phase]].CurrentResistor

		var ccf float64
		if c.CTType[phase] == "YHDC_SCT013" {
			ccf = CTTypes[c.CTType[phase]].CurrentClampFactor
		} else {
			ccf = 1.0 / (float64(c.CTTypePrimaryCurrent[phase]) / 100.0)
		}
		// fmt.Println("CalibrationfactorI: ", phase, "  ", c.CalibrationfactorI[phase])
		oc := CTTypes[c.CTType[phase]].OffsetCurrent
		// outcome = outcome - 7300
		current = ((((outcome * 0.3535) / rmsFactor) / cr) / ccf) * 100.0 * oc * c.CalibrationfactorI[phase]

//fmt.Printf("outcome=%f cr=%f CTTypePrimaryCurrent=%f ccf=%f oc=%f Calib=%f I=%f\n",outcome, cr, float64(c.CTTypePrimaryCurrent[phase]), ccf, oc, c.CalibrationfactorI[phase], current)


	} else {
		current = 0.0
	}

	return current
}

func ReadVoltage(d *i2c.Device, c *Config, phase Phase) (voltage float64, measureVoltage bool) {
	command := make([]byte, 2)
	switch phase {
	case PhaseA:
		command = []byte{0x43, 0xC1} // 0x43C1 (AVRMS; Voltage RMS phase A)
	case PhaseB:
		command = []byte{0x43, 0xC3} // 0x43C3 (BVRMS; Voltage RMS phase B)
	case PhaseC:
		command = []byte{0x43, 0xC5} // 0x43C5 (BVRMS; Voltage RMS phase C)
	default:
		panic(fmt.Errorf("Invalid phase %q", phase))
	}
	// fmt.Println("CalibrationfactorU: ", phase, "  ", c.CalibrationfactorU[phase])
	voltage = (float64(DeviceFetchInt(d, 4, command)) / 1e+4) * c.CalibrationfactorU[phase]

	measureVoltage = true
	if !c.MeasureVoltage[phase] {
		voltage = c.Voltage[phase]
		measureVoltage = false
	}

	return voltage, measureVoltage
}

func ReadActiveWatts(d *i2c.Device, c *Config, phase Phase) (watts float64) {
	command := make([]byte, 2)
	switch phase {
	case PhaseA:
		command = []byte{0xE5, 0x13} // 0xE513 (AWATT total active power phase A)
	case PhaseB:
		command = []byte{0xE5, 0x14} // 0xE514 (BWATT total active power phase B)
	case PhaseC:
		command = []byte{0xE5, 0x15} // 0xE515 (CWATT total active power phase C)
	default:
		panic(fmt.Errorf("Invalid phase %q", phase))
	}

	var pcf float64
	if c.CTType[phase] == "YHDC_SCT013" {
		pcf = 1.0
	} else {
		pcf = 200.0 / (float64(c.CTTypePrimaryCurrent[phase]))
	}

	outcome := float64(DeviceFetchInt(d, 4, command))
	if c.MeasureCurrent[phase] {
		watts = outcome * CTTypes[c.CTType[phase]].PowerCorrectionFactor / pcf
q :=  float64(ReadRegisterX(d, "BWATT"))
fmt.Printf("ZZZ %d outcome=%f (%f) PCF=%f pcf=%f watts=%f\n", phase, outcome, q, CTTypes[c.CTType[phase]].PowerCorrectionFactor, pcf, watts)
 

	} else {
		watts = 0.0
	}
	if c.CurrentDirection[phase] {
		watts *= -1
	}

	return watts
}

func ReadActiveEnergy(d *i2c.Device, c *Config, phase Phase) (energy float64) {
	command := make([]byte, 2)

	switch phase {
	case PhaseA:
		command = []byte{0xE4, 0x00} // 0xE4000 (AWATTHR total active energy phase A)
	case PhaseB:
		command = []byte{0xE4, 0x00} // 0xE4001 (BWATTHR total active energy phase B)
	case PhaseC:
		command = []byte{0xE4, 0x00} // 0xE4002 (CWATTHR total active energy phase C)
	default:
		panic(fmt.Errorf("Invalid phase %q", phase))
	}

	var pcf float64
	if c.CTType[phase] == "YHDC_SCT013" {
		pcf = 1.0
	} else {
		pcf = 200.0 / (float64(c.CTTypePrimaryCurrent[phase]))
	}

	outcome := float64(DeviceFetchInt(d, 4, command))

	energy = outcome / pcf

	// if c.CurrentDirection[phase] {
	// 	watts *= -1
	// }

	return energy
}

func ReadAngle(d *i2c.Device, c *Config, phase Phase) (angle float64) {
	command := make([]byte, 2)
	switch phase {
	case PhaseA:
		command = []byte{0xE6, 0x01} // 0xE601 (ANGLE0 cosphi an A)
	case PhaseB:
		command = []byte{0xE6, 0x02} // 0xE602 (ANGLE1 cosphi an B)
	case PhaseC:
		command = []byte{0xE6, 0x03} // 0xE603 (ANGLE2 cosphi an C)
	default:
		panic(fmt.Errorf("Invalid phase %q", phase))
	}

	if c.MeasureVoltage[phase] {
		outcome := float64(DeviceFetchInt(d, 2, command))
		angle = math.Cos(outcome * 360 * c.PowerFrequency / ade7878Clock * halfCircle)
		if c.CurrentDirection[phase] {
			angle *= -1
		}
	} else {
		angle = 1.0
	}

	return angle
}

func ReadFrequency(d *i2c.Device, c *Config, phase Phase) (frequency float64) {
	command := make([]byte, 2)
	switch phase {
	case PhaseA:
		command = []byte{0xE7, 0x00, 0x1C} // 0xE7001C MMODE-Register measure frequency at VA
	case PhaseB:
		command = []byte{0xE7, 0x00, 0x1D} // 0xE7001D MMODE-Register measure frequency at VB
	case PhaseC:
		command = []byte{0xE7, 0x00, 0x1E} // 0xE7001E MMODE-Register measure frequency at VC
	default:
		panic(fmt.Errorf("Invalid phase %q", phase))
	}

	err := d.Write(command) // MMODE-Register measure frequency
	if err != nil {
		panic(err)
	}
	// Make sure we capture 3 full cycles at ~50Hz, 4 cycles at ~60Hz.
	time.Sleep(70 * time.Millisecond)
	// 0xE607 (PERIOD)
	outcome := float64(DeviceFetchInt(d, 2, []byte{0xE6, 0x07}))
	frequency = ade7878Clock / (outcome + 1)

	return frequency
}

func ReadApparentPower(d *i2c.Device, c *Config, phase Phase) float64 {
	command := make([]byte, 2)
	switch phase {
	case PhaseA:
		command = []byte{0xE5, 0x19} // 0xE519 (AVA total apparent power phase A)
	case PhaseB:
		command = []byte{0xE5, 0x1A} // 0xE51A (BVA total apparent power phase B)
	case PhaseC:
		command = []byte{0xE5, 0x1B} // 0xE51B (CVA total apparent power phase C)
	default:
		panic(fmt.Errorf("Invalid phase %q", phase))
	}

	var pcf float64
	if c.CTType[phase] == "YHDC_SCT013" {
		pcf = 1.0
	} else {
		pcf = 200.0 / (float64(c.CTTypePrimaryCurrent[phase]))
	}

	if c.MeasureCurrent[phase] {
		outcome := float64(DeviceFetchInt(d, 4, command))
		return outcome * CTTypes[c.CTType[phase]].PowerCorrectionFactor / pcf * 1.14989234
	} else {
		return 0.0
	}
}

func ReadReactivePower(d *i2c.Device, c *Config, phase Phase) (rewatts float64) {
	command := make([]byte, 2)
	switch phase {
	case PhaseA:
		command = []byte{0xE5, 0x16} // 0xE516 (AVAR total reactive power phase A)
	case PhaseB:
		command = []byte{0xE5, 0x17} // 0xE517 (AVAR total reactive power phase B)
	case PhaseC:
		command = []byte{0xE5, 0x18} // 0xE518 (AVAR total reactive power phase C)
	default:
		panic(fmt.Errorf("Invalid phase %q", phase))
	}

	var pcf float64
	if c.CTType[phase] == "YHDC_SCT013" {
		pcf = 1.0
	} else {
		pcf = 200.0 / (float64(c.CTTypePrimaryCurrent[phase]))
	}

	outcome := float64(DeviceFetchInt(d, 4, command))
	if c.MeasureCurrent[phase] {
		rewatts = outcome * CTTypes[c.CTType[phase]].PowerCorrectionFactor / pcf * 2.560177029
	} else {
		rewatts = 0.0
	}
	if c.CurrentDirection[phase] {
		rewatts *= -1
	}

	return rewatts
}

func CalculatePowerFactor(c *Config, phase Phase, watts float64, voltAmps float64, voltAmpsReactive float64) float64 {
	powerFactor := watts / CTTypes[c.CTType[phase]].PowerCorrectionFactor / voltAmps
	if c.MeasureCurrent[phase] {
		if math.Signbit(voltAmpsReactive) {
			return powerFactor
		} else {
			return powerFactor * -1
		}
	} else {
		return 0.0
	}
}

func ReadPhase(d *i2c.Device, c *Config, p Phase, v *ADE7878Readout) {
	startTime := time.Now()

	// Measure current.
	v.Current[p] = ReadCurrent(d, c, p)

	// Neutral phase has no other updates.
	if p == PhaseN {
		logLine := fmt.Sprintf("ReadValues: %s phase: %s", time.Since(startTime), p)
		logLine += fmt.Sprintf("I: %g", v.Current[p])
		log.Debug(logLine)
		return
	}

	// Measure voltage.
	var measureVoltage bool
	v.Voltage[p], measureVoltage = ReadVoltage(d, c, p)

	// Measure active watts.
	if measureVoltage {
		v.ActiveWatts[p] = ReadActiveWatts(d, c, p)
	} else {
		v.ActiveWatts[p] = v.Current[p] * v.Voltage[p]
	}

	// Measure cosphi.
	v.CosPhi[p] = ReadAngle(d, c, p)

	// Measure apparent power (volt-amps).
	v.ApparentPower[p] = ReadApparentPower(d, c, p)

	// Measure reactive power (volt-ampere reactive).
	v.ReactivePower[p] = ReadReactivePower(d, c, p)

	// Measure active energy.
	v.ActiveEnergy[p] = ReadActiveEnergy(d, c, p)

	// Measure frequency.
	v.Frequency[p] = ReadFrequency(d, c, p)

	// Calculate power factor.
	v.PowerFactor[p] = CalculatePowerFactor(c, p, v.ActiveWatts[p], v.ApparentPower[p], v.ReactivePower[p])

	logLine := fmt.Sprintf("ReadValues: %s phase: %s", time.Since(startTime), p)
	logLine += fmt.Sprintf("I: %g  V: %g  P: %g ", v.Current[p], v.Voltage[p], v.ActiveWatts[p])
	logLine += fmt.Sprintf("COS: %g  F: %g  VA: %g  ", v.CosPhi[p], v.Frequency[p], v.ApparentPower[p])
	logLine += fmt.Sprintf("VAR: %g  PF: %g  WATTHR: %g  ", v.ReactivePower[p], v.PowerFactor[p], v.ActiveEnergy[p])
	log.Debug(logLine)
}

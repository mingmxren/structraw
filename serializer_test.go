package structraw

import (
	"log"
	"testing"
)

/*
le: LittleEndian
be: BigEndian
*/
type testStruct struct {
	AByteArray [4]byte
	AUInt8     uint8
	AUInt16    uint16 `structraw:"le"`
	AUInt32    uint32 `structraw:"le"`
	AUInt64    uint64 `structraw:"le"`
	AByteSlice []byte
}

func TestMarshal(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	ts1 := &testStruct{
		AByteArray: [4]byte{1, 2, 3, 4},
		AUInt8:     0xff,
		AUInt16:    0xff00,
		AUInt32:    0xffff0000,
		AUInt64:    0xffffffff00000000,
		AByteSlice: make([]byte, 10),
	}
	l, err := StructLen(ts1)
	log.Printf("len(ts1):%d", l)
	if err != nil {
		log.Fatalln(err)
	}
	for i := 0; i < len(ts1.AByteSlice); i++ {
		ts1.AByteSlice[i] = byte(i + 10)
	}
	log.Printf("ts1:%+v", ts1)
	ts2 := &testStruct{
		AByteSlice: make([]byte, 10),
	}
	b, err := Marshal(ts1)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("b(len:%d):%v", len(b), b)
	err = Unmarshal(b, ts2)
	log.Printf("ts2:%+v", ts2)
	if err != nil {
		log.Fatalln(err)
	}
}

func BenchmarkMarshal(b *testing.B) {
	ts1 := &testStruct{
		AByteArray: [4]byte{1, 2, 3, 4},
		AUInt8:     0xff,
		AUInt16:    0xff00,
		AUInt32:    0xffff0000,
		AUInt64:    0xffffffff00000000,
		AByteSlice: make([]byte, 10),
	}
	for i := 0; i < b.N; i++ {
		_, err := Marshal(ts1)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

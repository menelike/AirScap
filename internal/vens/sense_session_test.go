package vens

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
)

func TestSessionSense(t *testing.T) {
	for _, tc := range []struct {
		name           string
		key, asc, ascq byte
		short          bool
		wantErr        bool
		kind           ScanErrorKind
	}{
		{name: "clean"},
		{name: "complete", key: SenseKeyMediumError, asc: VendorASC, ascq: ASCQScanComplete},
		{name: "jam", key: SenseKeyMediumError, asc: VendorASC, ascq: ASCQPaperJam, wantErr: true, kind: ScanErrPaperJam},
		{name: "multi-feed", key: SenseKeyMediumError, asc: VendorASC, ascq: ASCQMultiFeed, wantErr: true, kind: ScanErrMultiFeed},
		{name: "truncated", short: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			go func() {
				defer server.Close()
				request := make([]byte, len(MarshalGetPageMetadata([8]byte{})))
				if _, err := io.ReadFull(server, request); err != nil {
					return
				}
				response := make([]byte, SenseDataOffset+18)
				response[SenseDataOffset+2] = tc.key
				response[SenseDataOffset+12] = tc.asc
				response[SenseDataOffset+13] = tc.ascq
				if tc.short {
					response = response[:SenseDataOffset+3]
				}
				binary.BigEndian.PutUint32(response, uint32(len(response)))
				server.Write(response)
			}()
			session := ScanSession{conn: client}
			err := session.readSense()
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr && !tc.short {
				var se *ScanError
				if !errors.As(err, &se) || se.Kind != tc.kind {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Replay the hardware status observed after the first duplex sheet. A clean
// sense response permits WAIT_FOR_SCAN; a real jam or other code must stop.
func TestNextPageStaleStatus(t *testing.T) {
	for _, tc := range []struct {
		name     string
		code     uint16
		status   uint32
		jam      bool
		wantWait bool
	}{
		{"stale", 0x2000, 0x8000, false, true},
		{"confirmed-jam", 0x2000, 0x8000, true, false},
		{"other-error", 0x0155, 0x8000, false, false},
		{"cover-open", 0x2000, ADFCoverOpenMask, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			observed := make(chan bool, 1)
			go func() {
				defer server.Close()
				exchange := func(request []byte, response []byte) bool {
					got := make([]byte, len(request))
					if _, err := io.ReadFull(server, got); err != nil {
						return false
					}
					if string(got) != string(request) {
						return false
					}
					binary.BigEndian.PutUint32(response, uint32(len(response)))
					_, err := server.Write(response)
					return err == nil
				}
				status := make([]byte, 48)
				binary.BigEndian.PutUint32(status[40:], tc.status)
				binary.BigEndian.PutUint32(status[44:], uint32(tc.code))
				if !exchange(MarshalGetStatus([8]byte{}), status) {
					observed <- false
					return
				}
				sense := make([]byte, 58)
				if tc.jam {
					sense[42] = SenseKeyMediumError
					sense[52] = VendorASC
					sense[53] = ASCQPaperJam
				}
				if !exchange(MarshalGetPageMetadata([8]byte{}), sense) {
					observed <- false
					return
				}
				if !tc.wantWait {
					observed <- false
					return
				}
				wait := make([]byte, 16)
				binary.BigEndian.PutUint32(wait[12:], 1)
				if !exchange(MarshalWaitForScan([8]byte{}), wait) {
					observed <- false
					return
				}
				complete := make([]byte, 58)
				complete[42] = SenseKeyMediumError
				complete[52] = VendorASC
				complete[53] = ASCQScanComplete
				observed <- exchange(MarshalGetPageMetadata([8]byte{}), complete)
			}()
			session := ScanSession{conn: client, physicalSheet: 1, sidesPerSheet: 2}
			_, err := session.NextPage()
			if tc.wantWait && err != io.EOF {
				t.Fatalf("expected completion after WAIT, got %v", err)
			}
			if !tc.wantWait && (err == nil || err == io.EOF) {
				t.Fatalf("expected scan error, got %v", err)
			}
			if got := <-observed; got != tc.wantWait {
				t.Fatalf("WAIT sent=%v, want %v", got, tc.wantWait)
			}
		})
	}
}

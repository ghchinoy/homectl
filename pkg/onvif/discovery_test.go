package onvif

import (
	"testing"
)

func TestDiscoveryProvider_Name(t *testing.T) {
	p := &DiscoveryProvider{}
	if got, want := p.Name(), "onvif"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func TestParseProbeMatch(t *testing.T) {
	tests := []struct {
		name      string
		xml       string
		ip        string
		wantName  string
		wantModel string
	}{
		{
			name: "full scopes with name and hardware",
			xml: `<e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope" xmlns:d="http://schemas.xmlsoap.org/ws/2004/08/discovery">
				<e:Body>
					<d:ProbeMatches>
						<d:ProbeMatch>
							<d:Scopes>onvif://www.onvif.org/type/video_encoder onvif://www.onvif.org/name/Front_Door onvif://www.onvif.org/hardware/ADC-V522IR</d:Scopes>
						</d:ProbeMatch>
					</d:ProbeMatches>
				</e:Body>
			</e:Envelope>`,
			ip:        "192.168.1.100",
			wantName:  "Front Door",
			wantModel: "ADC-V522IR",
		},
		{
			name: "multi-word name with underscores",
			xml: `<d:Scopes>onvif://www.onvif.org/name/North_Driveway_Perimeter onvif://www.onvif.org/hardware/ADC-V723</d:Scopes>`,
			ip:        "192.168.1.101",
			wantName:  "North Driveway Perimeter",
			wantModel: "ADC-V723",
		},
		{
			name: "hardware scope only",
			xml: `<d:Scopes>onvif://www.onvif.org/hardware/Generic-Cam</d:Scopes>`,
			ip:        "192.168.1.102",
			wantName:  "ONVIF Camera",
			wantModel: "Generic-Cam",
		},
		{
			name: "missing scopes tag",
			xml: `<e:Envelope><e:Body><d:ProbeMatches/></e:Body></e:Envelope>`,
			ip:        "192.168.1.103",
			wantName:  "ONVIF Camera",
			wantModel: "Unknown",
		},
		{
			name: "empty response",
			xml: "",
			ip:        "192.168.1.104",
			wantName:  "ONVIF Camera",
			wantModel: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := parseProbeMatch(tt.xml, tt.ip)
			if got, want := dev.ID, tt.ip; got != want {
				t.Errorf("ID got: %q, want: %q", got, want)
			}
			if got, want := dev.IP, tt.ip; got != want {
				t.Errorf("IP got: %q, want: %q", got, want)
			}
			if got, want := dev.Provider, "onvif"; got != want {
				t.Errorf("Provider got: %q, want: %q", got, want)
			}
			if got, want := dev.Type, "Camera"; got != want {
				t.Errorf("Type got: %q, want: %q", got, want)
			}
			if got, want := dev.Name, tt.wantName; got != want {
				t.Errorf("Name got: %q, want: %q", got, want)
			}
			if got, want := dev.Model, tt.wantModel; got != want {
				t.Errorf("Model got: %q, want: %q", got, want)
			}
		})
	}
}

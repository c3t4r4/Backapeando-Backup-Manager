package cpf

import "testing"

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "valid unformatted", input: "11144477735", want: "11144477735"},
		{name: "valid formatted", input: "111.444.777-35", want: "11144477735"},
		{name: "wrong first check digit", input: "11144477745", wantErr: true},
		{name: "wrong second check digit", input: "11144477734", wantErr: true},
		{name: "too short", input: "1234567890", wantErr: true},
		{name: "too long", input: "123456789012", wantErr: true},
		{name: "all repeated digits", input: "11111111111", wantErr: true},
		{name: "non-numeric stripped down to wrong length", input: "abc-765.528-39", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Validate(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Validate(%q) = %q, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("Validate(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestGenerateValidForTests(t *testing.T) {
	bases := []string{"123456789", "111111111", "000000000", "999999998"}
	for _, base := range bases {
		t.Run(base, func(t *testing.T) {
			generated, err := GenerateValidForTests(base)
			if err != nil {
				t.Fatalf("GenerateValidForTests(%q) unexpected error: %v", base, err)
			}
			if _, err := Validate(generated); err != nil {
				t.Fatalf("GenerateValidForTests(%q) = %q, which Validate rejects: %v", base, generated, err)
			}
		})
	}

	if _, err := GenerateValidForTests("12345"); err == nil {
		t.Fatal("GenerateValidForTests with wrong-length base should error")
	}
}

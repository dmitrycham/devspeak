package config

import "testing"

func TestLoadBot(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		userID  string
		want    BotConfig
		wantErr bool
	}{
		{name: "ok", token: "123:abc", userID: "42", want: BotConfig{Token: "123:abc", AllowedUserID: 42}},
		{name: "no token", token: "", userID: "42", wantErr: true},
		{name: "no user id", token: "123:abc", userID: "", wantErr: true},
		{name: "user id is not a number", token: "123:abc", userID: "me", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TELEGRAM_BOT_TOKEN", tt.token)
			t.Setenv("TELEGRAM_ALLOWED_USER_ID", tt.userID)

			got, err := LoadBot()
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadBot() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("LoadBot() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

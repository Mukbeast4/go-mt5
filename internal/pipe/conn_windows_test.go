//go:build windows

package pipe

import "testing"

func TestMT5PipeNameForPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{
			path: `C:\Program Files\MetaTrader 5\terminal64.exe`,
			want: `\\.\pipe\MT5.Terminal.781AEDD6B227148DB36F632AFAB710BBA441CCEA07ED9EF5BC7B94FAED25BD12`,
		},
		{
			path: `c:\program files\metatrader 5\terminal64.exe`,
			want: `\\.\pipe\MT5.Terminal.781AEDD6B227148DB36F632AFAB710BBA441CCEA07ED9EF5BC7B94FAED25BD12`,
		},
	}
	for _, c := range cases {
		got := mt5PipeNameForPath(c.path)
		if got != c.want {
			t.Errorf("mt5PipeNameForPath(%q):\n  got  %s\n  want %s", c.path, got, c.want)
		}
	}
}

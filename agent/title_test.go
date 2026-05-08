package agent

import "testing"

func TestDeriveTitle(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \n\t  ", ""},
		{"short ascii", "hi", "hi"},
		{"short cjk", "你好", "你好"},
		{"exactly 10", "abcdefghij", "abcdefghij"},
		{"over 10 ascii", "the quick brown fox jumps", "the quick "},
		{"over 10 cjk", "一二三四五六七八九十十一十二", "一二三四五六七八九十"},
		{"newlines become spaces", "hello\nworld", "hello worl"},
		{"crlf becomes spaces", "a\r\nb", "a  b"},
		{"trim then cut", "   hello world   ", "hello worl"},
		{"emoji rune", "🎉🎉🎉celebrate now!", "🎉🎉🎉celebra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveTitle(tc.in)
			if got != tc.want {
				t.Errorf("DeriveTitle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

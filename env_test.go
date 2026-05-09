package as_test

import (
	"context"
	"testing"

	"go.aledante.io/as"
)

func TestNormalizeEnvKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"docstring example", "my-Énv.key", "MY_ENV_KEY"},
		{"already normalized", "ALREADY_NORMALIZED", "ALREADY_NORMALIZED"},
		{"lowercase uppercased", "lowercase", "LOWERCASE"},
		{"single accent stripped", "café", "CAFE"},
		{"consecutive non-alnum collapsed", "hello!!world", "HELLO_WORLD"},
		{"leading and trailing non-alnum trimmed", "--leading-trailing--", "LEADING_TRAILING"},
		{"empty string", "", ""},
		{"only separators trims to empty", "___", ""},
		{"dashes and spaces collapse", "a - b   c", "A_B_C"},
		{"digits preserved", "a1b2", "A1B2"},
		{"digit-only", "12345", "12345"},
		{"mixed accents", "naïve café", "NAIVE_CAFE"},
		// Per the docstring "The resulting keys are POSIX-safe, consisting
		// only of [A-Z0-9_] and fully uppercase." Non-ASCII letters must
		// therefore be treated as non-alphanumeric and replaced with _.
		{"non-latin letters treated as non-alnum", "пример", ""},
		{"mixed ascii and non-latin", "abc-пример-123", "ABC_123"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := as.NormalizeEnvKey(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeEnvKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEnvPrefix_EmptyContext(t *testing.T) {
	t.Parallel()

	if got := as.EnvPrefix(context.Background()); got != "" {
		t.Errorf("EnvPrefix(background) = %q, want empty string", got)
	}
}

func TestEnvKey_NoPrefixNormalizesKey(t *testing.T) {
	t.Parallel()

	got := as.EnvKey(context.Background(), "my.key")
	if got != "MY_KEY" {
		t.Errorf("EnvKey(bg, %q) = %q, want %q", "my.key", got, "MY_KEY")
	}
}

func TestEnvKey_NormalizesAccents(t *testing.T) {
	t.Parallel()

	got := as.EnvKey(context.Background(), "café-clé")
	if got != "CAFE_CLE" {
		t.Errorf("EnvKey(bg, %q) = %q, want %q", "café-clé", got, "CAFE_CLE")
	}
}

func TestGetEnv_ReturnsValueAfterKeyNormalization(t *testing.T) {
	t.Setenv("MY_KEY", "val")

	if got := as.GetEnv(context.Background(), "my.key"); got != "val" {
		t.Errorf("GetEnv(bg, %q) = %q, want %q", "my.key", got, "val")
	}
}

func TestGetEnv_UnsetReturnsEmpty(t *testing.T) {
	if got := as.GetEnv(context.Background(), "UNSET_TEST_KEY_XYZ"); got != "" {
		t.Errorf("GetEnv for unset key = %q, want empty string", got)
	}
}

func TestLookupEnv_SetReturnsValueAndTrue(t *testing.T) {
	t.Setenv("MY_LOOKUP_KEY", "val")

	v, ok := as.LookupEnv(context.Background(), "my.lookup.key")
	if !ok || v != "val" {
		t.Errorf("LookupEnv = (%q, %v), want (%q, true)", v, ok, "val")
	}
}

func TestLookupEnv_UnsetReturnsEmptyAndFalse(t *testing.T) {
	v, ok := as.LookupEnv(context.Background(), "LOOKUP_ABSENT_XYZ_KEY")
	if ok || v != "" {
		t.Errorf("LookupEnv(absent) = (%q, %v), want (%q, false)", v, ok, "")
	}
}

type loadEnvTarget struct {
	Foo string `env:"FOO"`
	Bar int    `env:"BAR" envDefault:"42"`
}

func TestLoadEnv_ParsesStructFromEnvWithoutPrefix(t *testing.T) {
	t.Setenv("FOO", "hello")

	got, err := as.LoadEnv[loadEnvTarget](context.Background())
	if err != nil {
		t.Fatalf("LoadEnv returned error: %v", err)
	}
	if got.Foo != "hello" {
		t.Errorf("Foo = %q, want %q", got.Foo, "hello")
	}
	if got.Bar != 42 {
		t.Errorf("Bar (default) = %d, want %d", got.Bar, 42)
	}
}

func TestLoadEnv_DoesNotNormalizeTagKeys(t *testing.T) {
	// The docstring explicitly says LoadEnv does NOT normalize env keys.
	// A lowercase env var should therefore not match an uppercase `env:"FOO"` tag.
	t.Setenv("foo", "lower")
	t.Setenv("FOO", "UPPER")

	got, err := as.LoadEnv[loadEnvTarget](context.Background())
	if err != nil {
		t.Fatalf("LoadEnv returned error: %v", err)
	}
	if got.Foo != "UPPER" {
		t.Errorf("Foo = %q, want %q (tag is case-sensitive; lowercase env must be ignored)", got.Foo, "UPPER")
	}
}

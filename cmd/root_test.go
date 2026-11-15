package cmd

import (
	"testing"

	"github.com/roslaan001/ecsctl/pkg/aws"
)

func TestDebugFlagRegistered(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("debug")
	if f == nil {
		t.Fatal("expected --debug flag to be registered on rootCmd")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("expected --debug flag to be bool, got %s", f.Value.Type())
	}
}

func TestDebugPersistentPreRun(t *testing.T) {
	defer aws.SetDebug(false)

	debug = true
	if rootCmd.PersistentPreRun != nil {
		rootCmd.PersistentPreRun(rootCmd, nil)
	}
	debug = false

	// Test passes without error
}

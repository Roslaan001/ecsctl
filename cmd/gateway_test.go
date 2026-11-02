package cmd

import (
	"testing"
)

func TestGatewayCommandsRegistered(t *testing.T) {
	// Verify subcommand registration under create, describe, delete, list
	foundCreate := false
	for _, c := range createCmd.Commands() {
		if c.Name() == "gateway" {
			foundCreate = true
			break
		}
	}
	if !foundCreate {
		t.Errorf("create gateway subcommand not found on createCmd")
	}

	foundDescribe := false
	for _, c := range describeCmd.Commands() {
		if c.Name() == "gateway" {
			foundDescribe = true
			break
		}
	}
	if !foundDescribe {
		t.Errorf("describe gateway subcommand not found on describeCmd")
	}

	foundDelete := false
	for _, c := range deleteCmd.Commands() {
		if c.Name() == "gateway" {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Errorf("delete gateway subcommand not found on deleteCmd")
	}

	foundList := false
	for _, c := range listCmd.Commands() {
		if c.Name() == "gateways" {
			foundList = true
			break
		}
	}
	if !foundList {
		t.Errorf("list gateways subcommand not found on listCmd")
	}

	// Verify top-level gateway command and its subcommands
	foundRootGW := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "gateway" {
			foundRootGW = true
			break
		}
	}
	if !foundRootGW {
		t.Errorf("gateway command not found on rootCmd")
	}

	expectedSubcommands := map[string]bool{
		"create":       false,
		"describe":     false,
		"delete":       false,
		"list":         false,
		"associate":    false,
		"disassociate": false,
	}
	for _, c := range gatewayCmd.Commands() {
		expectedSubcommands[c.Name()] = true
	}
	for sub, found := range expectedSubcommands {
		if !found {
			t.Errorf("expected subcommand %q on gatewayCmd", sub)
		}
	}

	// Verify --gateway-port flag on createServiceCmd
	f := createServiceCmd.Flags().Lookup("gateway-port")
	if f == nil {
		t.Errorf("--gateway-port flag not found on createServiceCmd")
	}
}

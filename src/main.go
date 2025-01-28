package main

import (
	"DescribeNTSecurityDescriptor/ldap"
	"DescribeNTSecurityDescriptor/logger"
	"os"

	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/p0dalirius/goopts/parser"
	"github.com/p0dalirius/winacl/securitydescriptor"
)

var (
	// Configuration
	useLdaps bool
	debug    bool

	// Network settings
	domainController string
	ldapPort         int

	// Authentication details
	authDomain   string
	authUsername string
	authPassword string
	authHashes   string

	// Source values
	distinguishedName string

	sourceFileHex    string
	sourceFileBase64 string
	sourceFileRaw    string
	valueHex         string
	valueBase64      string
)

func parseArgs() {
	ap := parser.ArgumentsParser{Banner: "DescribeNTSecurityDescriptor - by Remi GASCOU (Podalirius) - v1.2"}

	// Configuration flags
	ap.NewBoolArgument(&debug, "-d", "--debug", false, "Debug mode.")

	// Source value
	group_sourceValues, err := ap.NewRequiredMutuallyExclusiveArgumentGroup("Source Values")
	if err != nil {
		fmt.Printf("[error] Error creating ArgumentGroup: %s\n", err)
	} else {
		group_sourceValues.NewStringArgument(&distinguishedName, "-D", "--distinguished-name", "", false, "Distinguished Name.")
		// File sources
		group_sourceValues.NewStringArgument(&sourceFileHex, "-fh", "--file-hex", "", false, "Path to file containing the hexadecimal string value of NTSecurityDescriptor.")
		group_sourceValues.NewStringArgument(&sourceFileBase64, "-fb", "--file-base64", "", false, "Path to file containing the base64 encoded value of NTSecurityDescriptor.")
		group_sourceValues.NewStringArgument(&sourceFileRaw, "-fr", "--file-raw", "", false, "Path to file containing the raw binary value of NTSecurityDescriptor.")
		// Value sources
		group_sourceValues.NewStringArgument(&valueHex, "-vh", "--value-hex", "", false, "Raw hexadecimal string value of NTSecurityDescriptor.")
		group_sourceValues.NewStringArgument(&valueBase64, "-vb", "--value-base64", "", false, "Raw base64 encoded value of NTSecurityDescriptor.")
	}

	group_ldapSettings, err := ap.NewArgumentGroup("LDAP Connection Settings")
	if err != nil {
		fmt.Printf("[error] Error creating ArgumentGroup: %s\n", err)
	} else {
		group_ldapSettings.NewStringArgument(&domainController, "-dc", "--dc-ip", "", false, "IP Address of the domain controller or KDC (Key Distribution Center) for Kerberos. If omitted, it will use the domain part (FQDN) specified in the identity parameter.")
		group_ldapSettings.NewTcpPortArgument(&ldapPort, "-P", "--port", 389, false, "Port number to connect to LDAP server.")
		group_ldapSettings.NewBoolArgument(&useLdaps, "-l", "--use-ldaps", false, "Use LDAPS instead of LDAP.")
	}

	group_auth, err := ap.NewArgumentGroup("Authentication")
	if err != nil {
		fmt.Printf("[error] Error creating ArgumentGroup: %s\n", err)
	} else {
		group_auth.NewStringArgument(&authDomain, "-d", "--domain", "", false, "Active Directory domain to authenticate to.")
		group_auth.NewStringArgument(&authUsername, "-u", "--username", "", false, "User to authenticate as.")
		group_auth.NewStringArgument(&authPassword, "-p", "--password", "", false, "Password to authenticate with.")
		group_auth.NewStringArgument(&authHashes, "-H", "--hashes", "", false, "NT/LM hashes, format is LMhash:NThash.")
	}

	ap.Parse()

	if useLdaps && !group_ldapSettings.LongNameToArgument["--port"].IsPresent() {
		ldapPort = 636
	}

	if len(distinguishedName) != 0 && (len(domainController) == 0 || len(authUsername) == 0 || len(authPassword) == 0) {
		logger.Warn("Error: Options --dc-ip, --username, --password are required when using --distinguished-name.")
		os.Exit(1)
	}
}

func main() {
	parseArgs()

	rawNtsdValue := []byte{}

	// Parsing input values for hex format
	if len(rawNtsdValue) == 0 && (len(sourceFileHex) != 0 || len(valueHex) != 0) {
		value_hex_string := ""
		if len(valueHex) != 0 {
			value_hex_string = valueHex
		}
		if len(sourceFileHex) != 0 {
			data, err := os.ReadFile(sourceFileHex)
			if err != nil {
				logger.Warn(fmt.Sprintf("Error reading file: %s", err))
				return
			}
			value_hex_string = string(data)
		}
		// Decoding the hex string
		if len(value_hex_string) != 0 {
			if len(value_hex_string)%2 == 1 {
				// encoding/hex: odd length hex string
				value_hex_string = value_hex_string + "0"
			}
			value, err := hex.DecodeString(value_hex_string)
			if err != nil {
				logger.Warn(fmt.Sprintf("Error decoding Hex value: %s", err))
				return
			} else {
				rawNtsdValue = value
			}
		}
	}

	// Parsing input values for base64 format
	if len(rawNtsdValue) == 0 && (len(sourceFileBase64) != 0 || len(valueBase64) != 0) {
		value_base64_string := ""
		if len(valueBase64) != 0 {
			value_base64_string = valueBase64
		}
		if len(sourceFileBase64) != 0 {
			data, err := os.ReadFile(sourceFileBase64)
			if err != nil {
				logger.Warn(fmt.Sprintf("Error reading file: %s", err))
				return
			}
			value_base64_string = string(data)
		}

		// Decoding the base64 string
		if len(value_base64_string) != 0 {
			value, err := base64.StdEncoding.DecodeString(value_base64_string)
			if err != nil {
				logger.Warn(fmt.Sprintf("Error decoding Base64 value: %s", err))
				return
			} else {
				rawNtsdValue = value
			}
		}
	}

	// Parsing input values for raw format
	if len(rawNtsdValue) == 0 && len(sourceFileRaw) != 0 {
		data, err := os.ReadFile(sourceFileRaw)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error reading file: %s", err))
			return
		}
		rawNtsdValue = data
	}

	if len(rawNtsdValue) == 0 && len(distinguishedName) != 0 {
		// Parsing input values for Distinguished Name
		if debug {
			if !useLdaps {
				logger.Debug(fmt.Sprintf("Connecting to remote ldap://%s:%d ...", domainController, ldapPort))
			} else {
				logger.Debug(fmt.Sprintf("Connecting to remote ldaps://%s:%d ...", domainController, ldapPort))
			}
		}
		ldapSession := ldap.Session{}
		ldapSession.InitSession(
			domainController,
			ldapPort,
			useLdaps,
			authDomain,
			authUsername,
			authPassword,
			debug,
		)
		connected := ldapSession.Connect()

		if connected {
			logger.Info(fmt.Sprintf("Connected as '%s\\%s'", authDomain, authUsername))

			query := fmt.Sprintf("(distinguishedName=%s)", distinguishedName)

			if debug {
				logger.Debug(fmt.Sprintf("LDAP query used: %s", query))
			}

			attributes := []string{"distinguishedName", "ntSecurityDescriptor"}
			ldapResults := ldap.QueryWholeSubtree(&ldapSession, "", query, attributes)

			for _, entry := range ldapResults {
				if debug {
					logger.Debug(fmt.Sprintf("| distinguishedName: %s", entry.GetAttributeValue("distinguishedName")))
				}
				rawNtsdValue = entry.GetEqualFoldRawAttributeValue("ntSecurityDescriptor")
			}
		} else {
			if debug {
				logger.Warn("Error: Could not create ldapSession.")
			}
		}
	}

	if len(rawNtsdValue) != 0 {
		ntSecurityDescriptor := securitydescriptor.NtSecurityDescriptor{}
		logger.Debug(fmt.Sprintf("| ntSecurityDescriptor: %s", hex.EncodeToString(rawNtsdValue)))
		ntSecurityDescriptor.Parse(rawNtsdValue)
		ntSecurityDescriptor.Describe(0)
	} else {
		logger.Warn("No NTSecurityDescriptor found in source values.")
	}
}

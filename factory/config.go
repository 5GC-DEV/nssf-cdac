// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

/*
 * NSSF Configuration Factory
 */

package factory

import (
	"strconv"

	"github.com/5GC-DEV/openapi-cdac/models"
	protos "github.com/omec-project/config5g/proto/sdcoreConfig"
	"github.com/omec-project/nssf/logger"
	utilLogger "github.com/omec-project/util/logger"
)

const (
	NSSF_EXPECTED_CONFIG_VERSION = "1.0.0"
)

type Config struct {
	Info          *Info              `yaml:"info"`
	Configuration *Configuration     `yaml:"configuration"`
	Logger        *utilLogger.Logger `yaml:"logger"`
	CfgLocation   string
	Subscriptions []Subscription `yaml:"subscriptions,omitempty"`
}

type Info struct {
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
}

const (
	NSSF_DEFAULT_IPV4     = "127.0.0.31"
	NSSF_DEFAULT_PORT     = "8000"
	NSSF_DEFAULT_PORT_INT = 8000
)

type Configuration struct {
	NssfName                 string                  `yaml:"nssfName,omitempty"`
	Sbi                      *Sbi                    `yaml:"sbi"`
	ServiceNameList          []models.ServiceName    `yaml:"serviceNameList"`
	NrfUri                   string                  `yaml:"nrfUri"`
	WebuiUri                 string                  `yaml:"webuiUri"`
	SupportedPlmnList        []models.PlmnId         `yaml:"supportedPlmnList,omitempty"`
	SupportedNssaiInPlmnList []SupportedNssaiInPlmn  `yaml:"supportedNssaiInPlmnList"`
	NsiList                  []NsiConfig             `yaml:"nsiList,omitempty"`
	AmfSetList               []AmfSetConfig          `yaml:"amfSetList"`
	AmfList                  []AmfConfig             `yaml:"amfList"`
	TaList                   []TaConfig              `yaml:"taList"`
	MappingListFromPlmn      []MappingFromPlmnConfig `yaml:"mappingListFromPlmn"`
}

type Sbi struct {
	Scheme models.UriScheme `yaml:"scheme"`
	TLS    *TLS             `yaml:"tls"`
	// Currently only support IPv4 and thus `Ipv4Addr` field shall not be empty
	RegisterIPv4 string `yaml:"registerIPv4,omitempty"` // IP that is registered at NRF.
	// IPv6Addr string `yaml:"ipv6Addr,omitempty"`
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port"`
}

type TLS struct {
	PEM string `yaml:"pem,omitempty"`
	Key string `yaml:"key,omitempty"`
}

type AmfConfig struct {
	NfId                           string                                  `yaml:"nfId"`
	SupportedNssaiAvailabilityData []models.SupportedNssaiAvailabilityData `yaml:"supportedNssaiAvailabilityData"`
}

type TaConfig struct {
	Tai                  *models.Tai               `yaml:"tai"`
	AccessType           *models.AccessType        `yaml:"accessType"`
	SupportedSnssaiList  []models.Snssai           `yaml:"supportedSnssaiList"`
	RestrictedSnssaiList []models.RestrictedSnssai `yaml:"restrictedSnssaiList,omitempty"`
}

type SupportedNssaiInPlmn struct {
	PlmnId              *models.PlmnId  `yaml:"plmnId"`
	SupportedSnssaiList []models.Snssai `yaml:"supportedSnssaiList"`
}

type NsiConfig struct {
	Snssai             *models.Snssai          `yaml:"snssai"`
	NsiInformationList []models.NsiInformation `yaml:"nsiInformationList"`
}

type AmfSetConfig struct {
	AmfSetId                       string                                  `yaml:"amfSetId"`
	AmfList                        []string                                `yaml:"amfList,omitempty"`
	NrfAmfSet                      string                                  `yaml:"nrfAmfSet,omitempty"`
	SupportedNssaiAvailabilityData []models.SupportedNssaiAvailabilityData `yaml:"supportedNssaiAvailabilityData"`
}

type MappingFromPlmnConfig struct {
	OperatorName    string                   `yaml:"operatorName,omitempty"`
	HomePlmnId      *models.PlmnId           `yaml:"homePlmnId"`
	MappingOfSnssai []models.MappingOfSnssai `yaml:"mappingOfSnssai"`
}

type Subscription struct {
	SubscriptionData *models.NssfEventSubscriptionCreateData `yaml:"subscriptionData"`
	SubscriptionId   string                                  `yaml:"subscriptionId"`
}

var ConfigPodTrigger chan bool

func init() {
	ConfigPodTrigger = make(chan bool)
}

func (c *Config) UpdateConfig(commChannel chan *protos.NetworkSliceResponse) bool {
	var minConfig bool

	for rsp := range commChannel {
		logger.GrpcLog.Infoln("Received updateConfig in the nssf app : ", rsp)

		// for thread safety
		ConfigLock.Lock()

		for _, ns := range rsp.NetworkSlice {
			logger.GrpcLog.Infoln("Network Slice Name ", ns.Name)

			if ns.Site == nil {
				continue
			}

			site := ns.Site
			logger.GrpcLog.Infoln("Site name ", site.SiteName)

			if site.Plmn == nil {
				logger.GrpcLog.Infoln("Plmn not present in the message ")
				continue
			}

			plmn := models.PlmnId{
				Mcc: site.Plmn.Mcc,
				Mnc: site.Plmn.Mnc,
			}

			logger.GrpcLog.Infof("PLMN: MCC=%s MNC=%s", plmn.Mcc, plmn.Mnc)

			// Parse NSSAI
			val, err := strconv.ParseInt(ns.Nssai.Sst, 10, 64)
			if err != nil {
				logger.GrpcLog.Errorf("Error parsing SST: %v", err)
				continue
			}

			nssai := models.Snssai{
				Sst: int32(val),
				Sd:  ns.Nssai.Sd,
			}

			logger.GrpcLog.Infof("Slice Sst=%d Sd=%s", nssai.Sst, nssai.Sd)

			// =========================
			// STEP 1: Update SupportedPlmn + NSSAI
			// =========================
			plmnIndex := -1

			for i, existingPlmn := range NssfConfig.Configuration.SupportedPlmnList {
				if existingPlmn.Mcc == plmn.Mcc && existingPlmn.Mnc == plmn.Mnc {
					plmnIndex = i
					break
				}
			}

			if plmnIndex >= 0 {
				// PLMN exists → update NSSAI list
				exists := false

				for _, existingSnssai := range NssfConfig.Configuration.SupportedNssaiInPlmnList[plmnIndex].SupportedSnssaiList {
					if existingSnssai.Sst == nssai.Sst && existingSnssai.Sd == nssai.Sd {
						exists = true
						break
					}
				}

				if !exists {
					NssfConfig.Configuration.SupportedNssaiInPlmnList[plmnIndex].SupportedSnssaiList = append(NssfConfig.Configuration.SupportedNssaiInPlmnList[plmnIndex].SupportedSnssaiList, nssai)
				}
			} else {
				// New PLMN
				NssfConfig.Configuration.SupportedPlmnList = append(NssfConfig.Configuration.SupportedPlmnList, plmn)
				newEntry := SupportedNssaiInPlmn{
					PlmnId:              &plmn,
					SupportedSnssaiList: []models.Snssai{nssai},
				}

				NssfConfig.Configuration.SupportedNssaiInPlmnList = append(NssfConfig.Configuration.SupportedNssaiInPlmnList, newEntry)
			}

			// =========================
			// STEP 2: Update MappingListFromPlmn
			// =========================
			mappingIndex := -1

			for i, mapping := range NssfConfig.Configuration.MappingListFromPlmn {
				if mapping.HomePlmnId != nil &&
					mapping.HomePlmnId.Mcc == plmn.Mcc &&
					mapping.HomePlmnId.Mnc == plmn.Mnc {
					mappingIndex = i
					break
				}
			}

			newMapping := models.MappingOfSnssai{
				HomeSnssai:    &nssai,
				ServingSnssai: &nssai, // 1:1 mapping (can customize later)
			}

			if mappingIndex >= 0 {
				// Update existing mapping
				exists := false

				for _, m := range NssfConfig.Configuration.MappingListFromPlmn[mappingIndex].MappingOfSnssai {
					if m.HomeSnssai.Sst == nssai.Sst && m.HomeSnssai.Sd == nssai.Sd {
						exists = true
						break
					}
				}

				if !exists {
					NssfConfig.Configuration.MappingListFromPlmn[mappingIndex].MappingOfSnssai = append(NssfConfig.Configuration.MappingListFromPlmn[mappingIndex].MappingOfSnssai, newMapping)
				}
			} else {
				// Create new mapping entry
				newEntry := MappingFromPlmnConfig{
					HomePlmnId:      &plmn,
					MappingOfSnssai: []models.MappingOfSnssai{newMapping},
				}

				NssfConfig.Configuration.MappingListFromPlmn = append(NssfConfig.Configuration.MappingListFromPlmn, newEntry)
			}
			// =========================
			// STEP 3: Update TA List
			// =========================
			for _, gnb := range site.Gnb {
				if gnb == nil {
					continue
				}
				if gnb.Tac == 0 {
					logger.GrpcLog.Warnln("TAC is 0 or not set in GNB")
					continue
				}

				// Convert TAC int32 → string
				tacStr := strconv.Itoa(int(gnb.Tac))

				tai := &models.Tai{
					PlmnId: &models.PlmnId{
						Mcc: site.Plmn.Mcc,
						Mnc: site.Plmn.Mnc,
					},
					Tac: tacStr,
				}

				// Set Access Type (most cases 3GPP)
				accessType := models.AccessType__3_GPP_ACCESS

				taConfig := TaConfig{
					Tai:                 tai,
					AccessType:          &accessType,
					SupportedSnssaiList: []models.Snssai{nssai},
					// Optional: keep empty unless needed
					RestrictedSnssaiList: nil,
				}

				exists := false

				for _, existingTai := range NssfConfig.Configuration.TaList {
					if existingTai.Tai.PlmnId.Mcc == tai.PlmnId.Mcc &&
						existingTai.Tai.PlmnId.Mnc == tai.PlmnId.Mnc &&
						existingTai.Tai.Tac == tai.Tac {
						exists = true
						break
					}
				}

				if !exists {
					NssfConfig.Configuration.TaList = append(NssfConfig.Configuration.TaList, taConfig)

					logger.GrpcLog.Infof("Added TA from GNB: MCC=%s MNC=%s TAC=%s",
						tai.PlmnId.Mcc, tai.PlmnId.Mnc, tai.Tac)
				}
			}
		}

		// =========================
		// Unlock after update
		// =========================
		ConfigLock.Unlock()

		hasConfig := len(NssfConfig.Configuration.SupportedPlmnList) > 0 && len(NssfConfig.Configuration.SupportedNssaiInPlmnList) > 0

		if hasConfig != minConfig {
			minConfig = hasConfig
			ConfigPodTrigger <- hasConfig
			logger.GrpcLog.Infof("Config trigger sent: %v", hasConfig)
		}
	}

	return true
}

func (c *Config) GetVersion() string {
	if c.Info != nil && c.Info.Version != "" {
		return c.Info.Version
	}
	return ""
}

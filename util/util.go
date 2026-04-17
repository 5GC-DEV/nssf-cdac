// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

/*
 * NSSF Utility
 */

package util

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/omec-project/nssf/factory"
	"github.com/omec-project/nssf/logger"
	"github.com/omec-project/openapi/models"
)

// Title in Problem Details for NSSF HTTP APIs
const (
	INVALID_REQUEST       = "Invalid request message framing"
	MALFORMED_REQUEST     = "Malformed request syntax"
	UNAUTHORIZED_CONSUMER = "Unauthorized NF service consumer"
	UNSUPPORTED_RESOURCE  = "Unsupported request resources"
)

// Check if a slice contains an element
func Contain(target interface{}, slice interface{}) bool {
	arr := reflect.ValueOf(slice)
	if arr.Kind() == reflect.Slice {
		for i := 0; i < arr.Len(); i++ {
			if reflect.DeepEqual(arr.Index(i).Interface(), target) {
				return true
			}
		}
	}
	return false
}

// Check whether UE's Home PLMN is configured/supported
func CheckSupportedHplmn(homePlmnId models.PlmnId) bool {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()

	// 1. Check Mapping List (Existing logic)
	for _, mappingFromPlmn := range factory.NssfConfig.Configuration.MappingListFromPlmn {
		if *mappingFromPlmn.HomePlmnId == homePlmnId {
			return true
		}
	}

	// 2. Check Supported S-NSSAIs in PLMN List
	// This covers the case where Slices are added dynamically (e.g. via GRPC)
	// but no specific mapping or whitelist entry was created.
	for _, supportedNssaiInPlmn := range factory.NssfConfig.Configuration.SupportedNssaiInPlmnList {
		if *supportedNssaiInPlmn.PlmnId == homePlmnId {
			return true
		}
	}

	// 3. Check Explicit Supported PLMN List (Whitelist)
	for _, supportedPlmn := range factory.NssfConfig.Configuration.SupportedPlmnList {
		if supportedPlmn.Mcc == homePlmnId.Mcc && supportedPlmn.Mnc == homePlmnId.Mnc {
			return true
		}
	}

	logger.Util.Warnf("no Home PLMN %+v in NSSF configuration", homePlmnId)
	return false
}

// Check whether UE's current TA is configured/supported
func CheckSupportedTa(tai models.Tai) bool {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	// 1. Check Global TaList (Primary Check)
	for _, taConfig := range factory.NssfConfig.Configuration.TaList {
		if taConfig.Tai != nil && compareTai(*taConfig.Tai, tai) {
			return true
		}
	}
	// 2. Check AMF List (Secondary Check)
	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		for _, supportedData := range amfConfig.SupportedNssaiAvailabilityData {
			if supportedData.Tai != nil && compareTai(*supportedData.Tai, tai) {
				return true
			}
		}
	}
	// 3. PLMN Fallback
	// If the exact TA is not explicitly listed (common in dynamic GRPC updates),
	// but the PLMN is supported and has Slices configured, we allow it.
	for _, supportedNssaiInPlmn := range factory.NssfConfig.Configuration.SupportedNssaiInPlmnList {
		if supportedNssaiInPlmn.PlmnId != nil &&
			supportedNssaiInPlmn.PlmnId.Mcc == tai.PlmnId.Mcc &&
			supportedNssaiInPlmn.PlmnId.Mnc == tai.PlmnId.Mnc {
			// Optional: Only log this in debug mode to avoid noise
			// logger.Util.Infof("TA %v allowed based on Supported PLMN fallback", tai)
			return true
		}
	}

	e, err := json.Marshal(tai)
	if err != nil {
		logger.Util.Errorf("marshal error in CheckSupportedTa: %+v", err)
	}
	logger.Util.Warnf("no TA %s in NSSF configuration", e)
	return false
}

// Helper function to compare TAI with Hex/Int flexibility
func compareTai(configTai, reqTai models.Tai) bool {
	// Check PLMN
	if configTai.PlmnId.Mcc != reqTai.PlmnId.Mcc || configTai.PlmnId.Mnc != reqTai.PlmnId.Mnc {
		return false
	}
	// Check TAC (Handle "0x" prefix and "000001" vs "1" mismatch)
	cfgTac := strings.TrimPrefix(configTai.Tac, "0x")
	reqTac := strings.TrimPrefix(reqTai.Tac, "0x")

	cfgVal, err1 := strconv.ParseInt(cfgTac, 16, 64)
	reqVal, err2 := strconv.ParseInt(reqTac, 16, 64)

	if err1 == nil && err2 == nil {
		return cfgVal == reqVal
	}
	// Fallback to string comparison if parsing fails
	return cfgTac == reqTac
}

// Check whether the given S-NSSAI is supported or not in PLMN
func CheckSupportedSnssaiInPlmn(snssai models.Snssai, plmnId models.PlmnId) bool {
	logger.Util.Infof("CheckSupportedSnssaiInPlmn: Start - SNSSAI: %+v, PLMN: %+v", snssai, plmnId)
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	if CheckStandardSnssai(snssai) {
		logger.Util.Infof("CheckSupportedSnssaiInPlmn: SNSSAI %+v is standard SNSSAI", snssai)
		// return true
		for _, supportedNssaiInPlmn := range factory.NssfConfig.Configuration.SupportedNssaiInPlmnList {
			if *supportedNssaiInPlmn.PlmnId == plmnId {
				for _, supportedSnssai := range supportedNssaiInPlmn.SupportedSnssaiList {
					if snssai.Sst == supportedSnssai.Sst {
						return true
					}
				}
				return false
			}
		}
	}
	logger.Util.Warnf("No supported S-NSSAI list found for PLMNID %+v in NSSF configuration", plmnId)
	return false
}

// Check whether S-NSSAIs in NSSAI are supported or not in PLMN
func CheckSupportedNssaiInPlmn(nssai []models.Snssai, plmnId models.PlmnId) bool {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, supportedNssaiInPlmn := range factory.NssfConfig.Configuration.SupportedNssaiInPlmnList {
		if *supportedNssaiInPlmn.PlmnId == plmnId {
			for _, snssai := range nssai {
				// Standard S-NSSAIs are supposed to be supported
				// If not, disable following check and be sure to add supported standard S-NSSAI(s) in configuration
				if CheckStandardSnssai(snssai) {
					continue
				}

				hitSupportedNssai := false
				for _, supportedSnssai := range supportedNssaiInPlmn.SupportedSnssaiList {
					if snssai == supportedSnssai {
						hitSupportedNssai = true
						break
					}
				}

				if !hitSupportedNssai {
					return false
				}
			}
			return true
		}
	}
	logger.Util.Warnf("no supported S-NSSAI list of PLMNID %+v in NSSF configuration", plmnId)
	return false
}

// Check whether S-NSSAI is supported or not at UE's current TA
func CheckSupportedSnssaiInTa(snssai models.Snssai, tai models.Tai) bool {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()

	logger.Util.Debugf("Input NSSAI: SST=%d SD=%s", snssai.Sst, snssai.Sd)

	if tai.PlmnId != nil {
		logger.Util.Debugf("Input TAI: MCC=%s MNC=%s TAC=%s",
			tai.PlmnId.Mcc, tai.PlmnId.Mnc, tai.Tac)
	} else {
		logger.Util.Warnf("TAI PLMN is nil")
	}

	// =========================
	// 1. Check Global TaList
	// =========================
	logger.Util.Debugf("Checking Global TaList...")

	for i, taConfig := range factory.NssfConfig.Configuration.TaList {
		if taConfig.Tai == nil {
			logger.Util.Warnf("TaList[%d]: TAI is nil", i)
			continue
		}

		if compareTai(*taConfig.Tai, tai) {
			logger.Util.Debugf("TaList[%d]: TAI MATCH FOUND", i)

			for j, supportedSnssai := range taConfig.SupportedSnssaiList {
				logger.Util.Debugf("TaList[%d]: Checking Supported NSSAI[%d]: SST=%d SD=%s",
					i, j, supportedSnssai.Sst, supportedSnssai.Sd)

				if supportedSnssai == snssai || supportedSnssai.Sst == snssai.Sst {
					logger.Util.Debugf("TaList[%d]: NSSAI MATCH FOUND → RETURN TRUE", i)
					return true
				}
			}

			logger.Util.Warnf("TaList[%d]: TAI matched but NSSAI NOT FOUND → RETURN FALSE", i)
			return false
		}
	}

	// =========================
	// 2. Check AMF List
	// =========================
	logger.Util.Debugf("Checking AMF SupportedNssaiAvailabilityData...")

	for i, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		logger.Util.Debugf("AMF[%d]: NfId=%s", i, amfConfig.NfId)
		for j, supportedData := range amfConfig.SupportedNssaiAvailabilityData {
			if supportedData.Tai == nil {
				logger.Util.Warnf("AMF[%d] Data[%d]: TAI is nil", i, j)
				continue
			}
			if compareTai(*supportedData.Tai, tai) {
				logger.Util.Debugf("AMF[%d] Data[%d]: TAI MATCH FOUND", i, j)

				for k, supportedSnssai := range supportedData.SupportedSnssaiList {
					logger.Util.Debugf("AMF[%d] Data[%d]: Checking NSSAI[%d]: SST=%d SD=%s",
						i, j, k, supportedSnssai.Sst, supportedSnssai.Sd)

					if supportedSnssai == snssai {
						logger.Util.Debugf("AMF[%d] Data[%d]: NSSAI MATCH FOUND → RETURN TRUE", i, j)
						return true
					}
				}
				logger.Util.Warnf("AMF[%d] Data[%d]: TAI matched but NSSAI NOT FOUND", i, j)
			}
		}
	}

	// =========================
	// 3. Standard NSSAI Fallback
	// =========================

	if CheckStandardSnssai(snssai) {
		logger.Util.Debugf("NSSAI is STANDARD (SST=%d, SD empty)", snssai.Sst)

		if tai.PlmnId != nil {
			plmnSupported := CheckSupportedSnssaiInPlmn(snssai, *tai.PlmnId)
			logger.Util.Debugf("PLMN support check result: %v", plmnSupported)

			if plmnSupported {
				logger.Util.Debugf("Standard NSSAI allowed via PLMN → RETURN TRUE")
				return true
			}
		} else {
			logger.Util.Warnf("Cannot check PLMN support → PLMN is nil")
		}
	} else {
		logger.Util.Debugf("NSSAI is NON-STANDARD → skipping fallback")
	}

	logger.Util.Warnf("No condition matched → RETURN FALSE")
	return false
}

// Check whether S-NSSAI is in SupportedNssaiAvailabilityData under the given TAI
func CheckSupportedNssaiAvailabilityData(snssai models.Snssai, tai models.Tai, s []models.SupportedNssaiAvailabilityData) bool {
	for i, data := range s {
		if data.Tai == nil {
			logger.Util.Warnf("Entry[%d]: Configured TAI is nil", i)
			continue
		}

		// Log CONFIGURED TAI (from NSSF config)
		if data.Tai.PlmnId != nil {
			logger.Util.Debugf("Entry[%d]: Configured TAI -> MCC=%s MNC=%s TAC=%s",
				i,
				data.Tai.PlmnId.Mcc,
				data.Tai.PlmnId.Mnc,
				data.Tai.Tac,
			)
		} else {
			logger.Util.Warnf("Entry[%d]: Configured TAI PLMN is nil", i)
		}

		// Log REQUESTED TAI (incoming from AMF)
		if tai.PlmnId != nil {
			logger.Util.Debugf("Entry[%d]: Requested TAI -> MCC=%s MNC=%s TAC=%s",
				i,
				tai.PlmnId.Mcc,
				tai.PlmnId.Mnc,
				tai.Tac,
			)
		} else {
			logger.Util.Warnf("Entry[%d]: Requested TAI PLMN is nil", i)
		}

		// Replace DeepEqual with manual comparison
		taiMatch := false

		if data.Tai.PlmnId != nil && tai.PlmnId != nil {
			if data.Tai.PlmnId.Mcc == tai.PlmnId.Mcc &&
				data.Tai.PlmnId.Mnc == tai.PlmnId.Mnc &&
				data.Tai.Tac == tai.Tac {
				taiMatch = true
			}
		}

		logger.Util.Debugf("Entry[%d]: TAI Match Result = %v", i, taiMatch)

		if !taiMatch {
			logger.Util.Warnf("Entry[%d]: TAI mismatch → Skipping NSSAI check", i)
			continue
		}

		// Check NSSAI
		nssaiMatch := CheckSnssaiInNssai(snssai, data.SupportedSnssaiList)

		logger.Util.Debugf("Entry[%d]: NSSAI Match = %v", i, nssaiMatch)

		if nssaiMatch {
			logger.Util.Debugf("Entry[%d]: MATCH FOUND (TAI + NSSAI)", i)
			return true
		}
	}
	logger.Util.Warnf("No matching TAI + NSSAI found")
	return false
}

// Check whether S-NSSAI is supported or not by the AMF at UE's current TA
func CheckSupportedSnssaiInAmfTa(snssai models.Snssai, nfId string, tai models.Tai) bool {
	logger.Util.Debugf("Input NF ID: %s", nfId)
	logger.Util.Debugf("Input SNSSAI: SST=%d SD=%s", snssai.Sst, snssai.Sd)
	if tai.PlmnId != nil {
		logger.Util.Debugf("Input TAI: MCC=%s MNC=%s TAC=%d", tai.PlmnId.Mcc, tai.PlmnId.Mnc, tai.Tac)
	} else {
		logger.Util.Warnf("TAI PLMN is nil")
	}
	logger.Util.Debugf("Configured AMF count: %d", len(factory.NssfConfig.Configuration.AmfList))
	for i, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		logger.Util.Debugf("Checking AMF[%d]: NfId=%s", i, amfConfig.NfId)
		if amfConfig.NfId == nfId {
			logger.Util.Debugf("Match found for NF ID: %s", nfId)
			if amfConfig.SupportedNssaiAvailabilityData == nil {
				logger.Util.Warnf("SupportedNssaiAvailabilityData is nil for AMF %s", nfId)
			} else {
				logger.Util.Debugf("SupportedNssaiAvailabilityData entries: %d",
					len(amfConfig.SupportedNssaiAvailabilityData))
			}
			result := CheckSupportedNssaiAvailabilityData(
				snssai,
				tai,
				amfConfig.SupportedNssaiAvailabilityData,
			)
			logger.Util.Debugf("Result from CheckSupportedNssaiAvailabilityData: %v", result)
			return result
		}
	}
	logger.Util.Warnf("No AMF found for NF ID: %s in NSSF configuration", nfId)
	return false
}

// Check whether all S-NSSAIs in Allowed NSSAI is supported by the AMF at UE's current TA
func CheckAllowedNssaiInAmfTa(allowedNssaiList []models.AllowedNssai, nfId string, tai models.Tai) bool {
	for _, allowedNssai := range allowedNssaiList {
		for _, allowedSnssai := range allowedNssai.AllowedSnssaiList {
			if CheckSupportedSnssaiInAmfTa(*allowedSnssai.AllowedSnssai, nfId, tai) {
				continue
			} else {
				return false
			}
		}
	}
	return true
}

// Check whether S-NSSAI is standard or non-standard value
// A standard S-NSSAI is only comprised of a standardized SST value and no SD
func CheckStandardSnssai(snssai models.Snssai) bool {
	if snssai.Sst >= 1 && snssai.Sst <= 3 && snssai.Sd == "" {
		return true
	}
	return false
}

// Check whether the NSSAI contains the specific S-NSSAI
func CheckSnssaiInNssai(targetSnssai models.Snssai, nssai []models.Snssai) bool {

	logger.Util.Debugf("Requested NSSAI -> SST=%d SD=%s",
		targetSnssai.Sst, targetSnssai.Sd)

	for i, snssai := range nssai {

		logger.Util.Debugf("Configured NSSAI[%d] -> SST=%d SD=%s",
			i, snssai.Sst, snssai.Sd)

		sstMatch := snssai.Sst == targetSnssai.Sst
		sdMatch := snssai.Sd == targetSnssai.Sd

		logger.Util.Infof("Comparison Result[%d] -> SST Match=%v, SD Match=%v",
			i, sstMatch, sdMatch)

		if sstMatch && sdMatch {
			logger.Util.Infof("NSSAI MATCH FOUND at index %d", i)
			logger.Util.Infof("==== CheckSnssaiInNssai END ====")
			return true
		}

		if !sstMatch || !sdMatch {
			logger.Util.Warnf("NSSAI mismatch at index %d -> Requested(SST=%d SD=%s) vs Configured(SST=%d SD=%s)",
				i,
				targetSnssai.Sst, targetSnssai.Sd,
				snssai.Sst, snssai.Sd,
			)
		}
	}

	logger.Util.Warnf("No matching NSSAI found in configured list")
	return false
}

func GetMappingOfPlmnFromConfig(homePlmnId models.PlmnId) []models.MappingOfSnssai {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()

	logger.CfgLog.Infof("GetMappingOfPlmnFromConfig called with HomePlmnId: MCC=%s, MNC=%s",
		homePlmnId.Mcc, homePlmnId.Mnc)

	if factory.NssfConfig.Configuration == nil {
		logger.CfgLog.Errorf("NSSF Config or Configuration is nil")
		return nil
	}
	logger.CfgLog.Infof("MappingListFromPlmn length: %d",
		len(factory.NssfConfig.Configuration.MappingListFromPlmn))

	if factory.NssfConfig.Configuration.MappingListFromPlmn == nil {
		logger.CfgLog.Warn("MappingListFromPlmn is nil")
	}

	for idx, mappingFromPlmn := range factory.NssfConfig.Configuration.MappingListFromPlmn {
		if mappingFromPlmn.HomePlmnId == nil {
			logger.CfgLog.Warnf("MappingListFromPlmn[%d] has nil HomePlmnId", idx)
			continue
		}

		logger.CfgLog.Infof("Checking MappingListFromPlmn[%d]: MCC=%s, MNC=%s",
			idx,
			mappingFromPlmn.HomePlmnId.Mcc,
			mappingFromPlmn.HomePlmnId.Mnc,
		)

		if *mappingFromPlmn.HomePlmnId == homePlmnId {
			logger.CfgLog.Infof("Match found for HomePlmnId at index %d", idx)

			if mappingFromPlmn.MappingOfSnssai == nil {
				logger.CfgLog.Warnf("MappingOfSnssai is nil for matched PLMN at index %d", idx)
			} else {
				logger.CfgLog.Infof("MappingOfSnssai count: %d", len(mappingFromPlmn.MappingOfSnssai))
			}

			return mappingFromPlmn.MappingOfSnssai
		}
	}

	logger.CfgLog.Infof("No mapping found for HomePlmnId: MCC=%s, MNC=%s",
		homePlmnId.Mcc, homePlmnId.Mnc)

	return nil
}

// Get NSI information list of the given S-NSSAI from configuration
func GetNsiInformationListFromConfig(snssai models.Snssai) []models.NsiInformation {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, nsiConfig := range factory.NssfConfig.Configuration.NsiList {
		if *nsiConfig.Snssai == snssai {
			return nsiConfig.NsiInformationList
		}
	}
	return nil
}

// Get Access Type of the given TAI from configuraion
func GetAccessTypeFromConfig(tai models.Tai) models.AccessType {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	logger.Util.Infof("Checking length[%d]", len(factory.NssfConfig.Configuration.TaList))
	for i, taConfig := range factory.NssfConfig.Configuration.TaList {
		logger.Util.Infof("Checking TA[%d]: %+v", i, taConfig.Tai)
		if reflect.DeepEqual(*taConfig.Tai, tai) {
			return *taConfig.AccessType
		}
	}
	e, err := json.Marshal(tai)
	if err != nil {
		logger.Util.Errorf("marshal error in GetAccessTypeFromConfig: %+v", err)
	}
	logger.Util.Warnf("no TA %s in NSSF configuration", e)
	return models.AccessType__3_GPP_ACCESS
}

// Get restricted S-NSSAI list of the given TAI from configuration
func GetRestrictedSnssaiListFromConfig(tai models.Tai) []models.RestrictedSnssai {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, taConfig := range factory.NssfConfig.Configuration.TaList {
		if reflect.DeepEqual(*taConfig.Tai, tai) {
			if len(taConfig.RestrictedSnssaiList) != 0 {
				return taConfig.RestrictedSnssaiList
			} else {
				return nil
			}
		}
	}
	e, err := json.Marshal(tai)
	if err != nil {
		logger.Util.Errorf("marshal error in GetRestrictedSnssaiListFromConfig: %+v", err)
	}
	logger.Util.Warnf("no TA %s in NSSF configuration", e)
	return nil
}

// Get authorized NSSAI availability data of the given NF ID and TAI from configuration
func AuthorizeOfAmfTaFromConfig(nfId string, tai models.Tai) (models.AuthorizedNssaiAvailabilityData, error) {
	var authorizedNssaiAvailabilityData models.AuthorizedNssaiAvailabilityData
	authorizedNssaiAvailabilityData.Tai = new(models.Tai)
	*authorizedNssaiAvailabilityData.Tai = tai

	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		if amfConfig.NfId == nfId {
			for _, supportedNssaiAvailabilityData := range amfConfig.SupportedNssaiAvailabilityData {
				if reflect.DeepEqual(*supportedNssaiAvailabilityData.Tai, tai) {
					authorizedNssaiAvailabilityData.SupportedSnssaiList = supportedNssaiAvailabilityData.SupportedSnssaiList
					authorizedNssaiAvailabilityData.RestrictedSnssaiList = GetRestrictedSnssaiListFromConfig(tai)

					// TODO: Sort the returned slice
					return authorizedNssaiAvailabilityData, nil
				}
			}
			e, err1 := json.Marshal(tai)
			if err1 != nil {
				logger.Util.Errorf("marshal error in AuthorizeOfAmfTaFromConfig: %+v", err1)
			}
			err := fmt.Errorf("no supported S-NSSAI list by AMF %s under TAI %s in NSSF configuration", nfId, e)
			return authorizedNssaiAvailabilityData, err
		}
	}
	err := fmt.Errorf("no AMF configuration of %s", nfId)
	return authorizedNssaiAvailabilityData, err
}

// Get all authorized NSSAI availability data of the given NF ID from configuration
func AuthorizeOfAmfFromConfig(nfId string) ([]models.AuthorizedNssaiAvailabilityData, error) {
	var authorizedNssaiAvailabilityDataList []models.AuthorizedNssaiAvailabilityData

	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		if amfConfig.NfId == nfId {
			for _, supportedNssaiAvailabilityData := range amfConfig.SupportedNssaiAvailabilityData {
				var authorizedNssaiAvailabilityData models.AuthorizedNssaiAvailabilityData
				authorizedNssaiAvailabilityData.Tai = new(models.Tai)
				*authorizedNssaiAvailabilityData.Tai = *supportedNssaiAvailabilityData.Tai
				authorizedNssaiAvailabilityData.SupportedSnssaiList = supportedNssaiAvailabilityData.SupportedSnssaiList
				authorizedNssaiAvailabilityData.RestrictedSnssaiList = GetRestrictedSnssaiListFromConfig(*authorizedNssaiAvailabilityData.Tai)

				authorizedNssaiAvailabilityDataList = append(authorizedNssaiAvailabilityDataList, authorizedNssaiAvailabilityData)
			}
			return authorizedNssaiAvailabilityDataList, nil
		}
	}
	err := fmt.Errorf("no AMF configuration of %s", nfId)
	return authorizedNssaiAvailabilityDataList, err
}

// Get authorized NSSAI availability data of the given TAI list from configuration
func AuthorizeOfTaListFromConfig(taiList []models.Tai) []models.AuthorizedNssaiAvailabilityData {
	var authorizedNssaiAvailabilityDataList []models.AuthorizedNssaiAvailabilityData

	for _, taConfig := range factory.NssfConfig.Configuration.TaList {
		for _, tai := range taiList {
			if reflect.DeepEqual(*taConfig.Tai, tai) {
				var authorizedNssaiAvailabilityData models.AuthorizedNssaiAvailabilityData
				authorizedNssaiAvailabilityData.Tai = new(models.Tai)
				*authorizedNssaiAvailabilityData.Tai = tai
				authorizedNssaiAvailabilityData.SupportedSnssaiList = taConfig.SupportedSnssaiList
				authorizedNssaiAvailabilityData.RestrictedSnssaiList = GetRestrictedSnssaiListFromConfig(tai)

				authorizedNssaiAvailabilityDataList = append(authorizedNssaiAvailabilityDataList, authorizedNssaiAvailabilityData)
			}
		}
	}
	return authorizedNssaiAvailabilityDataList
}

// Get supported S-NSSAI list of the given NF ID and TAI from configuration
func GetSupportedSnssaiListFromConfig(nfId string, tai models.Tai) []models.Snssai {
	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		if amfConfig.NfId == nfId {
			for _, supportedNssaiAvailabilityData := range amfConfig.SupportedNssaiAvailabilityData {
				if reflect.DeepEqual(*supportedNssaiAvailabilityData.Tai, tai) {
					return supportedNssaiAvailabilityData.SupportedSnssaiList
				}
			}
			return nil
		}
	}
	return nil
}

// Find target S-NSSAI mapping with serving S-NSSAIs from mapping of S-NSSAI(s)
func FindMappingWithServingSnssai(
	snssai models.Snssai, mappings []models.MappingOfSnssai,
) (models.MappingOfSnssai, bool) {
	for _, mapping := range mappings {
		if *mapping.ServingSnssai == snssai {
			return mapping, true
		}
	}
	return models.MappingOfSnssai{}, false
}

// Find target S-NSSAI mapping with home S-NSSAIs from mapping of S-NSSAI(s)
func FindMappingWithHomeSnssai(snssai models.Snssai, mappings []models.MappingOfSnssai) (models.MappingOfSnssai, bool) {
	for _, mapping := range mappings {
		if *mapping.HomeSnssai == snssai {
			return mapping, true
		}
	}
	return models.MappingOfSnssai{}, false
}

// Add Allowed S-NSSAI to Authorized Network Slice Info
func AddAllowedSnssai(allowedSnssai models.AllowedSnssai, accessType models.AccessType,
	authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	hitAllowedNssai := false
	allowedNssaiNum := 8
	for i := range authorizedNetworkSliceInfo.AllowedNssaiList {
		if authorizedNetworkSliceInfo.AllowedNssaiList[i].AccessType == accessType {
			hitAllowedNssai = true
			if len(authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList) == allowedNssaiNum {
				logger.Util.Infoln("unable to add a new Allowed S-NSSAI since already eight S-NSSAIs in Allowed NSSAI")
			} else {
				authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList = append(authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList, allowedSnssai)
			}
			break
		}
	}

	if !hitAllowedNssai {
		var allowedNssaiElement models.AllowedNssai
		allowedNssaiElement.AllowedSnssaiList = append(allowedNssaiElement.AllowedSnssaiList, allowedSnssai)
		allowedNssaiElement.AccessType = accessType

		authorizedNetworkSliceInfo.AllowedNssaiList = append(authorizedNetworkSliceInfo.AllowedNssaiList, allowedNssaiElement)
	}
}

// Add AMF information to Authorized Network Slice Info
func AddAmfInformation(tai models.Tai, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo) {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	if len(authorizedNetworkSliceInfo.AllowedNssaiList) == 0 {
		return
	}

	// Check if any AMF can serve the UE
	// That is, whether NSSAI of all Allowed S-NSSAIs is a subset of NSSAI supported by AMF

	// Find AMF Set that could serve UE from AMF Set list in configuration
	// Simply use the first applicable AMF set
	// TODO: Policies of AMF selection (e.g. load balance between AMF instances)
	for _, amfSetConfig := range factory.NssfConfig.Configuration.AmfSetList {
		hitAllowedNssai := true
		for _, allowedNssai := range authorizedNetworkSliceInfo.AllowedNssaiList {
			for _, allowedSnssai := range allowedNssai.AllowedSnssaiList {
				if CheckSupportedNssaiAvailabilityData(*allowedSnssai.AllowedSnssai,
					tai, amfSetConfig.SupportedNssaiAvailabilityData) {
					continue
				} else {
					hitAllowedNssai = false
					break
				}
			}
			if !hitAllowedNssai {
				break
			}
		}

		if !hitAllowedNssai {
			continue
		} else {
			// Add AMF Set to Authorized Network Slice Info
			if len(amfSetConfig.AmfList) != 0 {
				// List of candidate AMF(s) provided in configuration
				authorizedNetworkSliceInfo.CandidateAmfList = append(authorizedNetworkSliceInfo.CandidateAmfList, amfSetConfig.AmfList...)
			} else {
				// TODO: Possibly querying the NRF
				authorizedNetworkSliceInfo.TargetAmfSet = amfSetConfig.AmfSetId
				// The API URI of the NRF may be included if target AMF Set is included
				authorizedNetworkSliceInfo.NrfAmfSet = amfSetConfig.NrfAmfSet
			}
			return
		}
	}

	// No AMF Set in configuration can serve the UE
	// Find all candidate AMFs that could serve UE from AMF list in configuration
	hitAmf := false
	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		hitAllowedNssai := true
		for _, allowedNssai := range authorizedNetworkSliceInfo.AllowedNssaiList {
			for _, allowedSnssai := range allowedNssai.AllowedSnssaiList {
				if CheckSupportedNssaiAvailabilityData(*allowedSnssai.AllowedSnssai,
					tai, amfConfig.SupportedNssaiAvailabilityData) {
					continue
				} else {
					hitAllowedNssai = false
					break
				}
			}
			if !hitAllowedNssai {
				break
			}
		}

		if !hitAllowedNssai {
			continue
		} else {
			// Add AMF Set to Authorized Network Slice Info
			authorizedNetworkSliceInfo.CandidateAmfList = append(authorizedNetworkSliceInfo.CandidateAmfList, amfConfig.NfId)
			hitAmf = true
		}
	}

	if !hitAmf {
		logger.Util.Warnln("no candidate AMF or AMF Set can serve the UE")
	}
}

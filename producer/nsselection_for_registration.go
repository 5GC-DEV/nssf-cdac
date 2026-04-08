// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

/*
 * NSSF NS Selection
 *
 * NSSF Network Slice Selection Service
 */

package producer

import (
	"net/http"

	"github.com/omec-project/nssf/logger"
	"github.com/omec-project/nssf/plugin"
	"github.com/omec-project/nssf/util"
	"github.com/omec-project/openapi/models"
)

// Set Allowed NSSAI with Subscribed S-NSSAI(s) which are marked as default S-NSSAI(s)
func useDefaultSubscribedSnssai(
	param plugin.NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	var mappingOfSnssai []models.MappingOfSnssai
	if param.HomePlmnId != nil {
		// Find mapping of Subscribed S-NSSAI of UE's HPLMN to S-NSSAI in Serving PLMN from NSSF configuration
		mappingOfSnssai = util.GetMappingOfPlmnFromConfig(*param.HomePlmnId)

		if mappingOfSnssai == nil {
			// If no mapping found, check if HPLMN matches Serving PLMN (Roaming Status).
			// If TAI is present, check if PLMN ID matches Home PLMN ID
			isSamePlmn := false
			if param.Tai != nil &&
				param.Tai.PlmnId.Mcc == param.HomePlmnId.Mcc &&
				param.Tai.PlmnId.Mnc == param.HomePlmnId.Mnc {
				isSamePlmn = true
			}

			if !isSamePlmn {
				logger.Nsselection.Warnf("no S-NSSAI mapping of UE's HPLMN %+v in NSSF configuration", *param.HomePlmnId)
				return
			}
			// If it is the same PLMN, we proceed without 'mappingOfSnssai' (it stays nil/empty)
		}
	}

	for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
		if subscribedSnssai.DefaultIndication {
			// Subscribed S-NSSAI is marked as default S-NSSAI

			var mappingOfSubscribedSnssai models.Snssai
			// TODO: Compared with Restricted S-NSSAI list in configuration under roaming scenario
			if param.HomePlmnId != nil && !util.CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {
				targetMapping, found := util.FindMappingWithHomeSnssai(*subscribedSnssai.SubscribedSnssai, mappingOfSnssai)

				if !found {
					logger.Nsselection.Debugf("no mapping of Subscribed S-NSSAI %+v in PLMN %+v in NSSF configuration",
						*subscribedSnssai.SubscribedSnssai,
						*param.HomePlmnId)
					continue
				} else {
					mappingOfSubscribedSnssai = *targetMapping.ServingSnssai
				}
			} else {
				mappingOfSubscribedSnssai = *subscribedSnssai.SubscribedSnssai
			}

			if param.Tai != nil && !util.CheckSupportedSnssaiInTa(mappingOfSubscribedSnssai, *param.Tai) {
				continue
			}

			var allowedSnssaiElement models.AllowedSnssai
			allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
			*allowedSnssaiElement.AllowedSnssai = mappingOfSubscribedSnssai
			nsiInformationList := util.GetNsiInformationListFromConfig(mappingOfSubscribedSnssai)
			if nsiInformationList != nil {
				// TODO: `NsiInformationList` should be slice in `AllowedSnssai` instead of pointer of slice
				allowedSnssaiElement.NsiInformationList = append(allowedSnssaiElement.NsiInformationList,
					nsiInformationList...)
			}
			if param.HomePlmnId != nil && !util.CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {
				allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
				*allowedSnssaiElement.MappedHomeSnssai = *subscribedSnssai.SubscribedSnssai
			}

			// Default Access Type is set to 3GPP Access if no TAI is provided
			// TODO: Depend on operator implementation, it may also return S-NSSAIs in all valid Access Type if
			//       UE's Access Type could not be identified
			accessType := models.AccessType__3_GPP_ACCESS
			if param.Tai != nil {
				accessType = util.GetAccessTypeFromConfig(*param.Tai)
			}

			util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)
		}
	}
}

// Set Configured NSSAI with S-NSSAI(s) in Requested NSSAI which are marked as Default Configured NSSAI
func useDefaultConfiguredNssai(
	param plugin.NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	for _, requestedSnssai := range param.SliceInfoRequestForRegistration.RequestedNssai {
		// Check whether the Default Configured S-NSSAI is standard, which could be commonly decided by all roaming partners
		if !util.CheckStandardSnssai(requestedSnssai) {
			logger.Nsselection.Infof("s-nssai %+v in Requested NSSAI which based on Default Configured NSSAI is not standard",
				requestedSnssai)
			continue
		}

		// Check whether the Default Configured S-NSSAI is subscribed
		for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
			if requestedSnssai == *subscribedSnssai.SubscribedSnssai {
				var configuredSnssai models.ConfiguredSnssai
				configuredSnssai.ConfiguredSnssai = new(models.Snssai)
				*configuredSnssai.ConfiguredSnssai = requestedSnssai

				authorizedNetworkSliceInfo.ConfiguredNssai = append(authorizedNetworkSliceInfo.ConfiguredNssai, configuredSnssai)
				break
			}
		}
	}
}

// Set Configured NSSAI with Subscribed S-NSSAI(s)
func setConfiguredNssai(
	param plugin.NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	var mappingOfSnssai []models.MappingOfSnssai
	if param.HomePlmnId != nil {
		// Find mapping of Subscribed S-NSSAI of UE's HPLMN to S-NSSAI in Serving PLMN from NSSF configuration
		mappingOfSnssai = util.GetMappingOfPlmnFromConfig(*param.HomePlmnId)

		if mappingOfSnssai == nil {
			logger.Nsselection.Warnf("no S-NSSAI mapping of UE's HPLMN %+v in NSSF configuration", *param.HomePlmnId)
			return
		}
	}

	for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
		var mappingOfSubscribedSnssai models.Snssai
		if param.HomePlmnId != nil && !util.CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {
			targetMapping, found := util.FindMappingWithHomeSnssai(*subscribedSnssai.SubscribedSnssai, mappingOfSnssai)

			if !found {
				logger.Nsselection.Warnf("no mapping of Subscribed S-NSSAI %+v in PLMN %+v in NSSF configuration",
					*subscribedSnssai.SubscribedSnssai,
					*param.HomePlmnId)
				continue
			} else {
				mappingOfSubscribedSnssai = *targetMapping.ServingSnssai
			}
		} else {
			mappingOfSubscribedSnssai = *subscribedSnssai.SubscribedSnssai
		}

		if util.CheckSupportedSnssaiInPlmn(mappingOfSubscribedSnssai, *param.Tai.PlmnId) {
			var configuredSnssai models.ConfiguredSnssai
			configuredSnssai.ConfiguredSnssai = new(models.Snssai)
			*configuredSnssai.ConfiguredSnssai = mappingOfSubscribedSnssai
			if param.HomePlmnId != nil && !util.CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {
				configuredSnssai.MappedHomeSnssai = new(models.Snssai)
				*configuredSnssai.MappedHomeSnssai = *subscribedSnssai.SubscribedSnssai
			}

			authorizedNetworkSliceInfo.ConfiguredNssai = append(authorizedNetworkSliceInfo.ConfiguredNssai, configuredSnssai)
		}
	}
}

// Network slice selection for registration
// The function is executed when the IE, `slice-info-request-for-registration`, is provided in query parameters
func nsselectionForRegistration(param plugin.NsselectionQueryParameter,
	authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
	problemDetails *models.ProblemDetails,
) int {
	var status int
	if param.HomePlmnId != nil {
		// Check whether UE's Home PLMN is supported when UE is a roamer
		if !util.CheckSupportedHplmn(*param.HomePlmnId) {
			// If Home PLMN is not supported, we cannot select slices.
			// Return 403 Forbidden instead of 200 OK.
			logger.Nsselection.Warnf("Home PLMN %+v not supported. Returning 403.", *param.HomePlmnId)
			authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, param.SliceInfoRequestForRegistration.RequestedNssai...)

			*problemDetails = models.ProblemDetails{
				Title:  util.UNSUPPORTED_RESOURCE,
				Status: http.StatusForbidden,
				Detail: "Home PLMN is not supported",
				Cause:  "SNSSAI_NOT_SUPPORTED",
			}

			status = http.StatusForbidden
			return status
		}
	}

	if param.Tai != nil {
		// Check whether UE's current TA is supported when UE provides TAI
		if !util.CheckSupportedTa(*param.Tai) {
			// TA is not supported. We must return 403, not 200.
			logger.Nsselection.Warnf("TA %+v not supported. Returning 403.", *param.Tai)
			authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, param.SliceInfoRequestForRegistration.RequestedNssai...)

			// Populate Error Details
			*problemDetails = models.ProblemDetails{
				Title:  util.UNSUPPORTED_RESOURCE,
				Status: http.StatusForbidden,
				Detail: "Tracking Area (TA) is not supported",
				Cause:  "SNSSAI_NOT_SUPPORTED",
			}

			// Return 403
			status = http.StatusForbidden
			return status
		}
	}

	if param.SliceInfoRequestForRegistration.RequestMapping {
		// Based on TS 29.531 v15.2.0, when `requestMapping` is set to true, the NSSF shall return the VPLMN specific
		// mapped S-NSSAI values for the S-NSSAI values in `subscribedNssai`. But also `sNssaiForMapping` shall be
		// provided if `requestMapping` is set to true. In the implementation, the NSSF would return mapped S-NSSAIs
		// for S-NSSAIs in both `sNssaiForMapping` and `subscribedSnssai` if present

		if param.HomePlmnId == nil {
			problemDetail := "[Query Parameter] `home-plmn-id` should be provided when requesting VPLMN specific mapped S-NSSAI values"
			*problemDetails = models.ProblemDetails{
				Title:  util.INVALID_REQUEST,
				Status: http.StatusBadRequest,
				Detail: problemDetail,
				InvalidParams: []models.InvalidParam{
					{
						Param:  "home-plmn-id",
						Reason: problemDetail,
					},
				},
			}

			status = http.StatusBadRequest
			return status
		}

		mappingOfSnssai := util.GetMappingOfPlmnFromConfig(*param.HomePlmnId)

		if mappingOfSnssai != nil {
			// Find mappings for S-NSSAIs in `subscribedSnssai`
			for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
				if util.CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {

					logger.Nsselection.Infof("Standard NSSAI → allowing directly")

					var allowedSnssaiElement models.AllowedSnssai
					allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
					*allowedSnssaiElement.AllowedSnssai = *subscribedSnssai.SubscribedSnssai

					accessType := models.AccessType__3_GPP_ACCESS
					if param.Tai != nil {
						accessType = util.GetAccessTypeFromConfig(*param.Tai)
					}

					util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)

					continue
				}

				targetMapping, found := util.FindMappingWithHomeSnssai(*subscribedSnssai.SubscribedSnssai, mappingOfSnssai)

				if !found {
					logger.Nsselection.Debugf("no mapping of Subscribed S-NSSAI %+v in PLMN %+v in NSSF configuration",
						*subscribedSnssai.SubscribedSnssai,
						*param.HomePlmnId)
					continue
				} else {
					// Add mappings to Allowed NSSAI list
					var allowedSnssaiElement models.AllowedSnssai
					allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
					*allowedSnssaiElement.AllowedSnssai = *targetMapping.ServingSnssai
					allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
					*allowedSnssaiElement.MappedHomeSnssai = *subscribedSnssai.SubscribedSnssai

					// Default Access Type is set to 3GPP Access if no TAI is provided
					// TODO: Depend on operator implementation, it may also return S-NSSAIs in all valid Access Type if
					//       UE's Access Type could not be identified
					accessType := models.AccessType__3_GPP_ACCESS
					if param.Tai != nil {
						accessType = util.GetAccessTypeFromConfig(*param.Tai)
					}

					util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)
				}
			}

			// Find mappings for S-NSSAIs in `sNssaiForMapping`
			for _, snssai := range param.SliceInfoRequestForRegistration.SNssaiForMapping {
				if util.CheckStandardSnssai(snssai) {
					continue
				}

				targetMapping, found := util.FindMappingWithHomeSnssai(snssai, mappingOfSnssai)

				if !found {
					logger.Nsselection.Debugf("No mapping of Subscribed S-NSSAI %+v in PLMN %+v in NSSF configuration",
						snssai,
						*param.HomePlmnId)
					continue
				} else {
					// Add mappings to Allowed NSSAI list
					var allowedSnssaiElement models.AllowedSnssai
					allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
					*allowedSnssaiElement.AllowedSnssai = *targetMapping.ServingSnssai
					allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
					*allowedSnssaiElement.MappedHomeSnssai = snssai

					// Default Access Type is set to 3GPP Access if no TAI is provided
					// TODO: Depend on operator implementation, it may also return S-NSSAIs in all valid Access Type if
					//       UE's Access Type could not be identified
					accessType := models.AccessType__3_GPP_ACCESS
					if param.Tai != nil {
						accessType = util.GetAccessTypeFromConfig(*param.Tai)
					}

					util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)
				}
			}

			status = http.StatusOK
			return status
		} else {
			logger.Nsselection.Warnf("no S-NSSAI mapping of UE's HPLMN %+v in NSSF configuration", *param.HomePlmnId)

			status = http.StatusOK
			return status
		}
	}

	checkInvalidRequestedNssai := false
	logger.Nsselection.Infof("==== NS Selection Start ====")

	if len(param.SliceInfoRequestForRegistration.RequestedNssai) != 0 {

		logger.Nsselection.Infof("Requested NSSAI list: %+v",
			param.SliceInfoRequestForRegistration.RequestedNssai)

		if param.Tai != nil {
			logger.Nsselection.Infof("TAI: %+v", *param.Tai)
		}

		// 🔴 PLMN Check
		if param.Tai != nil &&
			!util.CheckSupportedNssaiInPlmn(param.SliceInfoRequestForRegistration.RequestedNssai, *param.Tai.PlmnId) {

			logger.Nsselection.Errorf("Requested NSSAI not supported in PLMN %+v",
				*param.Tai.PlmnId)

			*problemDetails = models.ProblemDetails{
				Title:  util.UNSUPPORTED_RESOURCE,
				Status: http.StatusForbidden,
				Detail: "S-NSSAI in Requested NSSAI is not supported in PLMN",
				Cause:  "SNSSAI_NOT_SUPPORTED",
			}

			return http.StatusForbidden
		}

		checkIfRequestAllowed := false

		for _, requestedSnssai := range param.SliceInfoRequestForRegistration.RequestedNssai {

			logger.Nsselection.Infof("---- Processing Requested NSSAI ----")
			logger.Nsselection.Infof("Requested: SST=%d SD=%s",
				requestedSnssai.Sst, requestedSnssai.Sd)

			// 🔴 TA Check
			if param.Tai != nil && !util.CheckSupportedSnssaiInTa(requestedSnssai, *param.Tai) {
				logger.Nsselection.Warnf("Requested NSSAI NOT supported in TA")

				authorizedNetworkSliceInfo.RejectedNssaiInTa =
					append(authorizedNetworkSliceInfo.RejectedNssaiInTa, requestedSnssai)

				continue
			} else {
				logger.Nsselection.Infof("Requested NSSAI supported in TA")
			}

			var mappingOfRequestedSnssai models.Snssai

			// 🔴 Mapping Decision
			if param.HomePlmnId != nil && !util.CheckStandardSnssai(requestedSnssai) {

				logger.Nsselection.Infof("Non-standard NSSAI → checking mapping")

				targetMapping, found := util.FindMappingWithServingSnssai(
					requestedSnssai,
					param.SliceInfoRequestForRegistration.MappingOfNssai,
				)

				if !found {
					logger.Nsselection.Warnf("Mapping NOT found for requested NSSAI")

					checkInvalidRequestedNssai = true
					authorizedNetworkSliceInfo.RejectedNssaiInPlmn =
						append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, requestedSnssai)

					continue
				} else {
					logger.Nsselection.Infof("Mapping FOUND → Home NSSAI: %+v",
						*targetMapping.HomeSnssai)

					mappingOfRequestedSnssai = *targetMapping.HomeSnssai
				}
			} else {
				logger.Nsselection.Infof("Standard NSSAI → no mapping needed")
				mappingOfRequestedSnssai = requestedSnssai
			}

			hitSubscription := false

			// 🔴 Subscription Match Loop
			for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {

				logger.Nsselection.Infof("Comparing Requested vs Subscribed")

				if subscribedSnssai.SubscribedSnssai == nil {
					logger.Nsselection.Warnf("SubscribedSnssai is nil")
					continue
				}

				logger.Nsselection.Infof("Requested: SST=%d SD=%s",
					mappingOfRequestedSnssai.Sst, mappingOfRequestedSnssai.Sd)

				logger.Nsselection.Infof("Subscribed: SST=%d SD=%s",
					subscribedSnssai.SubscribedSnssai.Sst,
					subscribedSnssai.SubscribedSnssai.Sd)

				if mappingOfRequestedSnssai.Sst == subscribedSnssai.SubscribedSnssai.Sst &&
					mappingOfRequestedSnssai.Sd == subscribedSnssai.SubscribedSnssai.Sd {

					logger.Nsselection.Infof("MATCH FOUND")

					hitSubscription = true

					var allowedSnssaiElement models.AllowedSnssai
					allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
					*allowedSnssaiElement.AllowedSnssai = requestedSnssai

					nsiInformationList := util.GetNsiInformationListFromConfig(requestedSnssai)
					if nsiInformationList != nil {
						logger.Nsselection.Infof("NSI Info found: %+v", nsiInformationList)
						allowedSnssaiElement.NsiInformationList =
							append(allowedSnssaiElement.NsiInformationList, nsiInformationList...)
					} else {
						logger.Nsselection.Warnf("No NSI Info found")
					}

					if param.HomePlmnId != nil && !util.CheckStandardSnssai(requestedSnssai) {
						allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
						*allowedSnssaiElement.MappedHomeSnssai =
							*subscribedSnssai.SubscribedSnssai
					}

					accessType := models.AccessType__3_GPP_ACCESS
					if param.Tai != nil {
						accessType = util.GetAccessTypeFromConfig(*param.Tai)
					}

					logger.Nsselection.Infof("Adding Allowed NSSAI → AccessType=%s", accessType)

					util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)

					checkIfRequestAllowed = true
					break

				} else {
					logger.Nsselection.Warnf("NO MATCH")
				}
			}

			if !hitSubscription {
				logger.Nsselection.Warnf("Requested NSSAI NOT in Subscribed list")

				checkInvalidRequestedNssai = true
				authorizedNetworkSliceInfo.RejectedNssaiInPlmn =
					append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, requestedSnssai)
			}
		}

		if !checkIfRequestAllowed {
			logger.Nsselection.Warnf("No requested NSSAI allowed → using default subscribed NSSAI")
			useDefaultSubscribedSnssai(param, authorizedNetworkSliceInfo)
		}

	} else {
		logger.Nsselection.Warnf("No Requested NSSAI → using default subscribed NSSAI")
		checkInvalidRequestedNssai = true
		useDefaultSubscribedSnssai(param, authorizedNetworkSliceInfo)
	}

	logger.Nsselection.Infof("Final Allowed NSSAI List: %+v",
		authorizedNetworkSliceInfo.AllowedNssaiList)

	logger.Nsselection.Infof("Final Configured NSSAI: %+v",
		authorizedNetworkSliceInfo.ConfiguredNssai)

	logger.Nsselection.Infof("==== NS Selection End ====")

	// 🔹 Check AMF support for Allowed NSSAI in given TAI
	if param.Tai != nil {

		logger.Nsselection.Infof("Checking Allowed NSSAI in AMF for given TAI...")
		logger.Nsselection.Infof("Input NF ID: %s", param.NfId)
		logger.Nsselection.Infof("Allowed NSSAI List before AMF check: %+v",
			authorizedNetworkSliceInfo.AllowedNssaiList)

		if !util.CheckAllowedNssaiInAmfTa(
			authorizedNetworkSliceInfo.AllowedNssaiList,
			param.NfId,
			*param.Tai,
		) {

			logger.Nsselection.Warnf("No matching AMF found for Allowed NSSAI in given TAI → Adding AMF info")

			util.AddAmfInformation(*param.Tai, authorizedNetworkSliceInfo)

			logger.Nsselection.Infof("AMF Information added to response")
		} else {
			logger.Nsselection.Infof("Allowed NSSAI is supported by AMF in given TAI")
		}
	} else {
		logger.Nsselection.Warnf("TAI is nil → Skipping AMF validation")
	}

	// 🔹 Handle Default Configured NSSAI Indication
	if param.SliceInfoRequestForRegistration.DefaultConfiguredSnssaiInd {

		logger.Nsselection.Infof("DefaultConfiguredSnssaiInd = TRUE → Using default configured NSSAI")

		useDefaultConfiguredNssai(param, authorizedNetworkSliceInfo)

		logger.Nsselection.Infof("Configured NSSAI after default selection: %+v",
			authorizedNetworkSliceInfo.ConfiguredNssai)

	} else if checkInvalidRequestedNssai {

		logger.Nsselection.Warnf("Invalid or no Requested NSSAI → Deriving Configured NSSAI from subscription")

		if param.Tai != nil {

			logger.Nsselection.Infof("Setting Configured NSSAI based on TAI and subscription")

			setConfiguredNssai(param, authorizedNetworkSliceInfo)

			logger.Nsselection.Infof("Configured NSSAI after processing: %+v",
				authorizedNetworkSliceInfo.ConfiguredNssai)

		} else {
			logger.Nsselection.Warnf("TAI is nil → Cannot derive Configured NSSAI")
		}
	} else {
		logger.Nsselection.Infof("Requested NSSAI valid → No need to derive Configured NSSAI")
	}

	// 🔹 Final Validation before response
	logger.Nsselection.Infof("Final Allowed NSSAI List: %+v",
		authorizedNetworkSliceInfo.AllowedNssaiList)

	logger.Nsselection.Infof("Final Configured NSSAI: %+v",
		authorizedNetworkSliceInfo.ConfiguredNssai)

	if len(authorizedNetworkSliceInfo.AllowedNssaiList) == 0 &&
		len(authorizedNetworkSliceInfo.ConfiguredNssai) == 0 {

		logger.Nsselection.Warnf("No S-NSSAI allowed or configured → Returning 403")

		*problemDetails = models.ProblemDetails{
			Title:  util.UNSUPPORTED_RESOURCE,
			Status: http.StatusForbidden,
			Detail: "No S-NSSAI found for the provided information",
			Cause:  "SNSSAI_NOT_SUPPORTED",
		}

		return http.StatusForbidden
	}

	logger.Nsselection.Infof("Valid NSSAI found → Returning 200 OK")

	status = http.StatusOK
	return status
}

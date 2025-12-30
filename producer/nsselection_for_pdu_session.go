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
	"fmt"
	"math/rand"
	"net/http"

	"github.com/omec-project/nssf/logger"
	"github.com/omec-project/nssf/plugin"
	"github.com/omec-project/nssf/util"
	"github.com/omec-project/openapi/models"
)

func selectNsiInformation(nsiInformationList []models.NsiInformation) models.NsiInformation {
	// TODO: Algorithm to select Network Slice Instance
	//       Take roaming indication into consideration

	// Randomly select a Network Slice Instance
	idx := rand.Intn(len(nsiInformationList))
	return nsiInformationList[idx]
}

// Network slice selection for PDU session
// The function is executed when the IE, `slice-info-for-pdu-session`, is provided in query parameters
func nsselectionForPduSession(param plugin.NsselectionQueryParameter,
	authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
	problemDetails *models.ProblemDetails,
) int {
	var status int
	if param.HomePlmnId != nil {
		logger.Nsselection.Infof("[nsselectionForPduSession] --inn----1")
		// Check whether UE's Home PLMN is supported when UE is a roamer
		if !util.CheckSupportedHplmn(*param.HomePlmnId) {
			logger.Nsselection.Infof(
				"[nsselectionForPduSession]-CheckSupportedHplmn-HomePlmnId S-NSSAI not supported in PLMN. Requested S-NSSAI={SST=%d, SD=%s}, PLMN={MCC=%s, MNC=%s}  Home PLMN={MCC=%s, MNC=%s}",
				param.SliceInfoRequestForPduSession.SNssai.Sst,
				param.SliceInfoRequestForPduSession.SNssai.Sd,
				param.Tai.PlmnId.Mcc,
				param.Tai.PlmnId.Mnc,
				param.HomePlmnId.Mcc,
				param.HomePlmnId.Mnc,
			)
			authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, *param.SliceInfoRequestForPduSession.SNssai)

			status = http.StatusOK
			return status
		}
	}

	if param.Tai != nil {
		logger.Nsselection.Infof("[nsselectionForPduSession] --inn----2")
		// Check whether UE's current TA is supported when UE provides TAI
		if !util.CheckSupportedTa(*param.Tai) {
			logger.Nsselection.Infof(
				"[nsselectionForPduSession]-CheckSupportedTa S-NSSAI not supported in PLMN. Requested S-NSSAI={SST=%d, SD=%s}, PLMN={MCC=%s, MNC=%s}",
				param.SliceInfoRequestForPduSession.SNssai.Sst,
				param.SliceInfoRequestForPduSession.SNssai.Sd,
				param.Tai.PlmnId.Mcc,
				param.Tai.PlmnId.Mnc)
			authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, *param.SliceInfoRequestForPduSession.SNssai)

			status = http.StatusOK
			return status
		}
	}

	if param.Tai != nil &&
		!util.CheckSupportedSnssaiInPlmn(*param.SliceInfoRequestForPduSession.SNssai, *param.Tai.PlmnId) {
		logger.Nsselection.Infof(
			"[nsselectionForPduSession] --CheckSupportedSnssaiInPlmn-----S-NSSAI not supported in PLMN. Requested S-NSSAI={SST=%d, SD=%s}, PLMN={MCC=%s, MNC=%s}",
			param.SliceInfoRequestForPduSession.SNssai.Sst,
			param.SliceInfoRequestForPduSession.SNssai.Sd,
			param.Tai.PlmnId.Mcc,
			param.Tai.PlmnId.Mnc,
		)
		// Return ProblemDetails indicating S-NSSAI is not supported
		// TODO: Based on TS 23.501 V15.2.0, if the Requested NSSAI includes an S-NSSAI that is not valid in the
		//       Serving PLMN, the NSSF may derive the Configured NSSAI for Serving PLMN
		*problemDetails = models.ProblemDetails{
			Title:  util.UNSUPPORTED_RESOURCE,
			Status: http.StatusForbidden,
			Detail: "S-NSSAI in Requested NSSAI is not supported in PLMN",
			Cause:  "SNSSAI_NOT_SUPPORTED",
		}

		status = http.StatusForbidden
		return status
	}

	if param.HomePlmnId != nil {
		logger.Nsselection.Infof("[nsselectionForPduSession] --inn----3")
		logger.Nsselection.Infof(
			"[nsselectionForPduSession] --HomePlmnId----12 S-NSSAI not supported in PLMN. Requested S-NSSAI={SST=%d, SD=%s}, PLMN={MCC=%s, MNC=%s} ",
			param.SliceInfoRequestForPduSession.SNssai.Sst,
			param.SliceInfoRequestForPduSession.SNssai.Sd,
			param.Tai.PlmnId.Mcc,
			param.Tai.PlmnId.Mnc,
		)
		if param.SliceInfoRequestForPduSession.RoamingIndication == models.RoamingIndication_NON_ROAMING {
			problemDetail := "`home-plmn-id` is provided, which contradicts `roamingIndication`:'NON_ROAMING'"
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
	} else {
		if param.SliceInfoRequestForPduSession.RoamingIndication != models.RoamingIndication_NON_ROAMING {
			logger.Nsselection.Infof(
				"[nsselectionForPduSession] --RoamingIndication----22 S-NSSAI not supported in PLMN. Requested S-NSSAI={SST=%d, SD=%s}, PLMN={MCC=%s, MNC=%s} ",
				param.SliceInfoRequestForPduSession.SNssai.Sst,
				param.SliceInfoRequestForPduSession.SNssai.Sd,
				param.Tai.PlmnId.Mcc,
				param.Tai.PlmnId.Mnc,
			)
			problemDetail := fmt.Sprintf("`home-plmn-id` is not provided, which contradicts `roamingIndication`:'%s'",
				string(param.SliceInfoRequestForPduSession.RoamingIndication))
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
	}

	if param.Tai != nil && !util.CheckSupportedSnssaiInTa(*param.SliceInfoRequestForPduSession.SNssai, *param.Tai) {
		logger.Nsselection.Infof("[nsselectionForPduSession] --inn----333")
		// Requested S-NSSAI does not supported in UE's current TA
		// Add it to Rejected NSSAI in TA
		authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, *param.SliceInfoRequestForPduSession.SNssai)
		status = http.StatusOK
		return status
	}

	nsiInformationList := util.GetNsiInformationListFromConfig(*param.SliceInfoRequestForPduSession.SNssai)

	if nsiInformationList == nil {
		logger.Nsselection.Infof("[nsselectionForPduSession] --inn----444")
		*authorizedNetworkSliceInfo = models.AuthorizedNetworkSliceInfo{}
	} else {
		logger.Nsselection.Infof("[nsselectionForPduSession] --inn----5555")
		nsiInformation := selectNsiInformation(nsiInformationList)
		authorizedNetworkSliceInfo.NsiInformation = new(models.NsiInformation)
		*authorizedNetworkSliceInfo.NsiInformation = nsiInformation
	}

	return http.StatusOK
}

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
func nsselectionForPduSession(
	param plugin.NsselectionQueryParameter,
	authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
	problemDetails *models.ProblemDetails,
) int {
	logger.Nsselection.Infof("[nsselectionForPduSession] Entered nsselectionForPduSession")
	var status int
	if param.HomePlmnId != nil {
		logger.Nsselection.Infof("[nsselectionForPduSession] HomePlmnId provided")
		if !util.CheckSupportedHplmn(*param.HomePlmnId) {
			logger.Nsselection.Infof("[nsselectionForPduSession] Unsupported Home PLMN")
			authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn,
				*param.SliceInfoRequestForPduSession.SNssai)
			*problemDetails = models.ProblemDetails{
				Title:  util.UNSUPPORTED_RESOURCE,
				Status: http.StatusForbidden,
				Detail: "Home PLMN is not supported",
				Cause:  "SNSSAI_NOT_SUPPORTED",
			}
			status = http.StatusForbidden
			logger.Nsselection.Infof("[nsselectionForPduSession] ----1  Returning status: %d", status)
			return status
		}
	}

	if param.Tai != nil {
		logger.Nsselection.Infof("[nsselectionForPduSession] TAI provided")
		if !util.CheckSupportedTa(*param.Tai) {
			logger.Nsselection.Infof("[nsselectionForPduSession] Unsupported TA")
			authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa,
				*param.SliceInfoRequestForPduSession.SNssai)

			status = http.StatusOK
			logger.Nsselection.Infof("[nsselectionForPduSession] ----2  Returning status: %d", status)
			return status
		}
	}

	if param.Tai != nil {
		logger.Nsselection.Infof("[nsselectionForPduSession] TAI provided- for slice")

		if !util.CheckSupportedSnssaiInPlmn(
			*param.SliceInfoRequestForPduSession.SNssai,
			*param.Tai.PlmnId) {
			logger.Nsselection.Infof("[nsselectionForPduSession] S-NSSAI not supported in PLMN")
			*problemDetails = models.ProblemDetails{
				Title:  util.UNSUPPORTED_RESOURCE,
				Status: http.StatusForbidden,
				Detail: "S-NSSAI in Requested NSSAI is not supported in PLMN",
				Cause:  "SNSSAI_NOT_SUPPORTED",
			}
			status = http.StatusForbidden
			logger.Nsselection.Infof("[nsselectionForPduSession] ----3  Returning status: %d", status)
			return status
		}
	}

	if param.HomePlmnId != nil {
		if param.SliceInfoRequestForPduSession.RoamingIndication ==
			models.RoamingIndication_NON_ROAMING {
			logger.Nsselection.Infof("[nsselectionForPduSession] Contradiction: home-plmn-id + NON_ROAMING")
			problemDetail := "`home-plmn-id` is provided, which contradicts `roamingIndication`:'NON_ROAMING'"
			*problemDetails = models.ProblemDetails{
				Title:  util.INVALID_REQUEST,
				Status: http.StatusForbidden,
				Detail: problemDetail,
				Cause:  "SNSSAI_NOT_SUPPORTED",
				InvalidParams: []models.InvalidParam{
					{
						Param:  "home-plmn-id",
						Reason: problemDetail,
					},
				},
			}
			status = http.StatusForbidden
			logger.Nsselection.Infof("[nsselectionForPduSession] ----4 Returning status: %d", status)
			return status
		}
	} else {
		if param.SliceInfoRequestForPduSession.RoamingIndication !=
			models.RoamingIndication_NON_ROAMING {
			logger.Nsselection.Infof("[nsselectionForPduSession] Contradiction: roamingIndication without home-plmn-id")
			problemDetail := fmt.Sprintf(
				"`home-plmn-id` is not provided, which contradicts `roamingIndication`:'%s'",
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
			logger.Nsselection.Infof("[nsselectionForPduSession] ----5 Returning status: %d", status)
			return status
		}
	}

	if param.Tai != nil &&
		!util.CheckSupportedSnssaiInTa(
			*param.SliceInfoRequestForPduSession.SNssai,
			*param.Tai) {
		logger.Nsselection.Infof("[nsselectionForPduSession] S-NSSAI not supported in TA")
		authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa,
			*param.SliceInfoRequestForPduSession.SNssai)
		status = http.StatusOK
		logger.Nsselection.Infof("[nsselectionForPduSession] Returning status:---6 %d", status)
		return status
	}

	logger.Nsselection.Infof("[nsselectionForPduSession] Fetching NSI Information from config")

	nsiInformationList := util.GetNsiInformationListFromConfig(
		*param.SliceInfoRequestForPduSession.SNssai)

	if nsiInformationList == nil {
		logger.Nsselection.Infof("[nsselectionForPduSession] No NSI Information found")
		*authorizedNetworkSliceInfo = models.AuthorizedNetworkSliceInfo{}
	} else {
		logger.Nsselection.Infof("[nsselectionForPduSession] Selecting NSI Information")
		nsiInformation := selectNsiInformation(nsiInformationList)
		authorizedNetworkSliceInfo.NsiInformation = new(models.NsiInformation)
		*authorizedNetworkSliceInfo.NsiInformation = nsiInformation
	}

	logger.Nsselection.Infof("[nsselectionForPduSession] Returning status:---7 %d", http.StatusOK)
	return http.StatusOK
}

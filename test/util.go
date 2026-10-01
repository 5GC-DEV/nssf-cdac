// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

/*
 * NSSF Testing Utility
 */

package test

import (
	"reflect"

	"github.com/5GC-DEV/openapi-cdac/models"
)

func CheckAuthorizedNetworkSliceInfo(target models.AuthorizedNetworkSliceInfo, expectList []models.AuthorizedNetworkSliceInfo) bool {
	for _, expectElement := range expectList {
		if reflect.DeepEqual(target, expectElement) {
			return true
		}
	}
	return false
}

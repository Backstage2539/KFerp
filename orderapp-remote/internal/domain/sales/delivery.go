package sales

import "strings"

func IsNonCourierShipMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case "pickup", "local_delivery":
		return true
	}
	return false
}

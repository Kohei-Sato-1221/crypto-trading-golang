package app

import (
	bitflyerApp "github.com/Kohei-Sato-1221/crypto-trading-golang/go/app/bitflyerApp"
)

// StartBfService はbitflyerAppのStartBfServiceを呼び出すラッパー。
func StartBfService() {
	bitflyerApp.StartBfService()
}

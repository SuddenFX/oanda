// Copyright 2014 Tjerk Santegoeds
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package oanda_test

import (
	"github.com/santegoeds/oanda"
	"sync"
	"time"

	"gopkg.in/check.v1"
)

type TestTranSuite struct {
	c *oanda.Client
}

var _ = check.Suite(&TestTranSuite{})

func (ts *TestTranSuite) SetUpTest(c *check.C) {
	var err error
	ts.c, err = newSandboxClientWithAccount()
	c.Assert(err, check.IsNil)
}

func (ts *TestTranSuite) TestTransactionApi(c *check.C) {
	trans, err := ts.c.Transactions()
	c.Assert(err, check.IsNil)
	c.Assert(trans, check.HasLen, 2)

	m := make(map[string]bool)
	for _, tran := range trans {
		m[tran.Type()] = true

		switch tran.Type() {
		case "CREATE":
			_, ok := tran.AsAccountCreate()
			c.Assert(ok, check.Equals, true)
		case "TRANSFER_FUNDS":
			_, ok := tran.AsTransferFunds()
			c.Assert(ok, check.Equals, true)
		}
	}

	c.Log(m)

	_, ok := m["CREATE"]
	c.Assert(ok, check.Equals, true)

	_, ok = m["TRANSFER_FUNDS"]
	c.Assert(ok, check.Equals, true)

	tran, err := ts.c.Transaction(trans[0].TranId())
	c.Assert(err, check.IsNil)
	c.Log(tran)
	c.Assert(tran.Type(), check.Equals, "TRANSFER_FUNDS")

	tfTran, ok := tran.AsTransferFunds()
	c.Assert(ok, check.Equals, true)
	c.Assert(tfTran.Amount(), check.Equals, 100000.0)
}

type TestEventsServerSuite struct {
	TestTranSuite
}

func (ts *TestTranSuite) TestEventsServer(c *check.C) {
	es, err := ts.c.NewEventsServer(ts.c.AccountId)
	c.Assert(err, check.IsNil)

	wg := sync.WaitGroup{}

	t := time.AfterFunc(5*time.Minute, func() {
		es.Stop()
		c.Fail()
	})

	expiry := time.Now().Add(24 * time.Hour)

	wg.Add(1)
	go func() {
		err := es.Run(func(accountId int, tran *oanda.Transaction) {
			c.Log(accountId, tran)

			es.Stop()
			t.Stop()

			c.Assert(accountId, check.Equals, tran.AccountId())
			c.Assert(tran.Type(), check.Equals, "LIMIT_ORDER_CREATE")

			ocTran, ok := tran.AsOrderCreate()
			c.Assert(ok, check.Equals, true)
			c.Assert(ocTran.Instrument(), check.Equals, "EUR_USD")
			c.Assert(ocTran.Side(), check.Equals, "buy")
			c.Assert(ocTran.Units(), check.Equals, 1)
			c.Assert(ocTran.Price(), check.Equals, 0.75)
			c.Assert(ocTran.Expiry().Equal(expiry.Truncate(time.Second)), check.Equals, true)
			c.Assert(ocTran.Reason(), check.Equals, "CLIENT_REQUEST")
		})
		c.Assert(err, check.IsNil)
		wg.Done()
	}()

	time.Sleep(5 * time.Second)

	ts.c.NewOrder(oanda.Ot_Limit, oanda.Ts_Buy, 1, "eur_usd", 0.75, expiry)
	wg.Wait()
}

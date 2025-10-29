package transit

import (
	"testing"

	"github.com/nimbolus/terraform-backend/pkg/client/vault/vaulttest"
	"github.com/nimbolus/terraform-backend/pkg/kms/util"
)

func TestKMS(t *testing.T) {
	v := vaulttest.NewIfIntegrationTest(t)

	util.KMSTest(t, NewVaultTransit(v, "transit", "terraform-backend"))
}

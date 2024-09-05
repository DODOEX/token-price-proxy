package service_test

import (
	"testing"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/stretchr/testify/assert"
)

func TestGetAppTokenByToken(t *testing.T) {
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	repo := repository.NewAppTokenRepository(db, redis)
	tokenService := service.NewAppTokenService(repo)

	// 在测试前清理测试数据
	db.DB.Exec("DELETE FROM app_tokens WHERE token LIKE 'test-%'")

	// 添加测试数据
	expectedToken := &schema.AppToken{
		Name:  "TestApp",
		Token: "test-token",
		Rate:  1.0,
	}
	err := tokenService.AddAppToken(expectedToken)
	assert.NoError(t, err)

	// 测试获取数据
	token, err := tokenService.GetAppTokenByToken("test-token")
	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, expectedToken, token)

	// 清理测试数据
	db.DB.Exec("DELETE FROM app_tokens WHERE token = ?", "test-token")
}

func TestGetAllAppTokens(t *testing.T) {
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	repo := repository.NewAppTokenRepository(db, redis)
	tokenService := service.NewAppTokenService(repo)

	// 在测试前清理测试数据
	db.DB.Exec("DELETE FROM app_tokens WHERE token LIKE 'test-%'")

	// 添加测试数据
	expectedTokens := []schema.AppToken{
		{Name: "TestApp1", Token: "test-token1", Rate: 1.0},
		{Name: "TestApp2", Token: "test-token2", Rate: 2.0},
	}
	for _, token := range expectedTokens {
		err := tokenService.AddAppToken(&token)
		assert.NoError(t, err)
	}

	// 测试获取所有数据
	tokens, err := tokenService.GetAllAppTokens()
	assert.NoError(t, err)
	if !tokensAreEqual(expectedTokens, tokens) {
		t.Errorf("Expected tokens %v, but got %v", expectedTokens, tokens)
	}
	// 清理测试数据
	db.DB.Exec("DELETE FROM app_tokens WHERE token LIKE 'test-%'")
}

func TestAddAppToken(t *testing.T) {
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	repo := repository.NewAppTokenRepository(db, redis)
	tokenService := service.NewAppTokenService(repo)

	// 在测试前清理测试数据
	db.DB.Exec("DELETE FROM app_tokens WHERE token LIKE 'test-%'")

	// 测试添加数据
	newToken := &schema.AppToken{
		Name:  "TestAddApp",
		Token: "test-add-token",
		Rate:  1.5,
	}
	err := tokenService.AddAppToken(newToken)
	assert.NoError(t, err)

	// 验证数据是否添加成功
	token, err := tokenService.GetAppTokenByToken("test-add-token")
	assert.NoError(t, err)
	assert.Equal(t, newToken, token)

	err = tokenService.UpdateAppToken(&schema.AppToken{
		Name:  "TestAddApp",
		Token: "test-add-token",
		Rate:  15,
	})
	assert.NoError(t, err)

	err = tokenService.DeleteAppToken("test-add-token")
	assert.NoError(t, err)

	// 清理测试数据
	db.DB.Exec("DELETE FROM app_tokens WHERE token = ?", "test-add-token")
}
func tokensAreEqual(expected, actual []schema.AppToken) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].Name != actual[i].Name ||
			expected[i].Token != actual[i].Token ||
			expected[i].Rate != actual[i].Rate {
			return false
		}
	}
	return true
}

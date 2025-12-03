package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// OrderRepositoryTestSuite тестовый suite для PostgreSQL repository
type OrderRepositoryTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	repo    OrderRepository
	sqlDB   *sql.DB
}

func TestOrderRepositorySuite(t *testing.T) {
	suite.Run(t, new(OrderRepositoryTestSuite))
}

func (s *OrderRepositoryTestSuite) SetupTest() {
	var err error
	s.sqlDB, s.mock, err = sqlmock.New()
	require.NoError(s.T(), err)

	dialector := postgres.New(postgres.Config{
		Conn:       s.sqlDB,
		DriverName: "postgres",
	})

	s.db, err = gorm.Open(dialector, &gorm.Config{})
	require.NoError(s.T(), err)

	s.repo = NewOrderRepository(s.db)
}

func (s *OrderRepositoryTestSuite) TearDownTest() {
	s.sqlDB.Close()
}

// ===================== GetByID Tests =====================

func (s *OrderRepositoryTestSuite) TestGetByID_Success() {
	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()
	createdAt := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "total_price", "delivery_price", "currency", "status", "created_at"}).
		AddRow(orderID, userID, 110.0, 10.0, "USD", "pending", createdAt)

	s.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID).
		WillReturnRows(rows)

	// Act
	order, err := s.repo.GetByID(ctx, orderID)

	// Assert
	s.NoError(err)
	s.NotNil(order)
	s.Equal(orderID, order.ID)
	s.Equal(userID, order.UserID)
	s.Equal(110.0, order.TotalPrice)
	s.Equal(10.0, order.DeliveryPrice)
	s.Equal("USD", order.Currency)
	s.Equal(entity.OrderStatusPending, order.Status)

	s.NoError(s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	orderID := uuid.New()

	s.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID).
		WillReturnError(gorm.ErrRecordNotFound)

	// Act
	order, err := s.repo.GetByID(ctx, orderID)

	// Assert
	s.Error(err)
	s.Nil(order)
	s.Contains(err.Error(), "order not found")

	s.NoError(s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestGetByID_DBError() {
	ctx := context.Background()
	orderID := uuid.New()

	s.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID).
		WillReturnError(sql.ErrConnDone)

	// Act
	order, err := s.repo.GetByID(ctx, orderID)

	// Assert
	s.Error(err)
	s.Nil(order)
	s.Contains(err.Error(), "failed to get order")

	s.NoError(s.mock.ExpectationsWereMet())
}

// ===================== Update Tests =====================

func (s *OrderRepositoryTestSuite) TestUpdate_Success() {
	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    200.0,
		DeliveryPrice: 20.0,
		Currency:      "RUB",
		Status:        entity.OrderStatusConfirmed,
		CreatedAt:     time.Now(),
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	// Act
	err := s.repo.Update(ctx, order)

	// Assert
	s.NoError(err)
	s.NoError(s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestUpdate_NotFound() {
	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    200.0,
		DeliveryPrice: 20.0,
		Currency:      "RUB",
		Status:        entity.OrderStatusConfirmed,
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected
	s.mock.ExpectCommit()

	// Act
	err := s.repo.Update(ctx, order)

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "not found")

	s.NoError(s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestUpdate_DBError() {
	ctx := context.Background()
	order := &entity.Order{
		ID:       uuid.New(),
		UserID:   uuid.New(),
		Currency: "USD",
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnError(sql.ErrConnDone)
	s.mock.ExpectRollback()

	// Act
	err := s.repo.Update(ctx, order)

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "failed to update")

	s.NoError(s.mock.ExpectationsWereMet())
}

// ===================== UpdateDeliveryAndTotal Tests =====================

func (s *OrderRepositoryTestSuite) TestUpdateDeliveryAndTotal_Success() {
	ctx := context.Background()
	orderID := uuid.New()

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WithArgs(10.0, 110.0, orderID). // delivery_price, total_price, id
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	// Act
	err := s.repo.UpdateDeliveryAndTotal(ctx, orderID, 10.0, 110.0)

	// Assert
	s.NoError(err)
	s.NoError(s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestUpdateDeliveryAndTotal_NotFound() {
	ctx := context.Background()
	orderID := uuid.New()

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WithArgs(10.0, 110.0, orderID).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected
	s.mock.ExpectCommit()

	// Act
	err := s.repo.UpdateDeliveryAndTotal(ctx, orderID, 10.0, 110.0)

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "not found")

	s.NoError(s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestUpdateDeliveryAndTotal_DBError() {
	ctx := context.Background()
	orderID := uuid.New()

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WithArgs(10.0, 110.0, orderID).
		WillReturnError(sql.ErrConnDone)
	s.mock.ExpectRollback()

	// Act
	err := s.repo.UpdateDeliveryAndTotal(ctx, orderID, 10.0, 110.0)

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "failed to update")

	s.NoError(s.mock.ExpectationsWereMet())
}

// ===================== UpdateOrderWithCurrency Tests =====================

func (s *OrderRepositoryTestSuite) TestUpdateOrderWithCurrency_Success() {
	ctx := context.Background()
	orderID := uuid.New()

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WithArgs("RUB", 912.3, 10035.3, orderID). // currency, delivery_price, total_price, id
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	// Act
	err := s.repo.UpdateOrderWithCurrency(ctx, orderID, 912.3, 10035.3, "RUB")

	// Assert
	s.NoError(err)
	s.NoError(s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestUpdateOrderWithCurrency_NotFound() {
	ctx := context.Background()
	orderID := uuid.New()

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WithArgs("RUB", 912.3, 10035.3, orderID).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected
	s.mock.ExpectCommit()

	// Act
	err := s.repo.UpdateOrderWithCurrency(ctx, orderID, 912.3, 10035.3, "RUB")

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "not found")

	s.NoError(s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestUpdateOrderWithCurrency_DBError() {
	ctx := context.Background()
	orderID := uuid.New()

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WithArgs("RUB", 912.3, 10035.3, orderID).
		WillReturnError(sql.ErrConnDone)
	s.mock.ExpectRollback()

	// Act
	err := s.repo.UpdateOrderWithCurrency(ctx, orderID, 912.3, 10035.3, "RUB")

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "failed to update")

	s.NoError(s.mock.ExpectationsWereMet())
}

// ===================== NewOrderRepository Tests =====================

func TestNewOrderRepository(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	dialector := postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	})

	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)

	// Act
	repo := NewOrderRepository(db)

	// Assert
	assert.NotNil(t, repo)
}

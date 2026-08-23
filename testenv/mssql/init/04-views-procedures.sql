-- 04-views-procedures.sql
USE SalesDB;
GO

-- 1. View: Order Summary with Totals
IF OBJECT_ID('sales.v_OrderSummary', 'V') IS NOT NULL DROP VIEW sales.v_OrderSummary;
GO
CREATE VIEW sales.v_OrderSummary
AS
SELECT 
    o.OrderID,
    o.OrderDate,
    o.OrderStatus,
    c.CustomerID,
    c.CompanyName,
    c.Country,
    CONCAT(e.FirstName, ' ', e.LastName) AS SalesRep,
    COUNT(oi.OrderItemID) AS ItemCount,
    SUM(oi.UnitPrice * oi.Quantity * (1.0 - oi.Discount)) AS SubTotal,
    o.Freight,
    SUM(oi.UnitPrice * oi.Quantity * (1.0 - oi.Discount)) + o.Freight AS GrandTotal
FROM sales.Orders o
INNER JOIN sales.Customers c ON o.CustomerID = c.CustomerID
LEFT JOIN hr.Employees e ON o.EmployeeID = e.EmployeeID
LEFT JOIN sales.OrderItems oi ON o.OrderID = oi.OrderID
GROUP BY 
    o.OrderID, o.OrderDate, o.OrderStatus, c.CustomerID, c.CompanyName, c.Country,
    e.FirstName, e.LastName, o.Freight;
GO

-- 2. View: Product Catalog with Stock Status
IF OBJECT_ID('inventory.v_ProductCatalog', 'V') IS NOT NULL DROP VIEW inventory.v_ProductCatalog;
GO
CREATE VIEW inventory.v_ProductCatalog
AS
SELECT 
    p.ProductID,
    p.ProductName,
    c.CategoryName,
    s.CompanyName AS SupplierName,
    p.UnitPrice,
    p.UnitsInStock,
    p.UnitsOnOrder,
    p.ReorderLevel,
    CASE 
        WHEN p.UnitsInStock <= p.ReorderLevel THEN 'REORDER NEEDED'
        WHEN p.UnitsInStock = 0 THEN 'OUT OF STOCK'
        ELSE 'IN STOCK'
    END AS StockStatus,
    p.IsDiscontinued
FROM inventory.Products p
INNER JOIN inventory.Categories c ON p.CategoryID = c.CategoryID
LEFT JOIN inventory.Suppliers s ON p.SupplierID = s.SupplierID;
GO

-- 3. Stored Procedure: Customer Order History
IF OBJECT_ID('sales.sp_GetCustomerOrderHistory', 'P') IS NOT NULL DROP PROCEDURE sales.sp_GetCustomerOrderHistory;
GO
CREATE PROCEDURE sales.sp_GetCustomerOrderHistory
    @CustomerID NVARCHAR(10)
AS
BEGIN
    SET NOCOUNT ON;

    SELECT 
        o.OrderID,
        o.OrderDate,
        o.OrderStatus,
        p.ProductName,
        oi.UnitPrice,
        oi.Quantity,
        oi.Discount,
        (oi.UnitPrice * oi.Quantity * (1.0 - oi.Discount)) AS LineTotal
    FROM sales.Orders o
    INNER JOIN sales.OrderItems oi ON o.OrderID = oi.OrderID
    INNER JOIN inventory.Products p ON oi.ProductID = p.ProductID
    WHERE o.CustomerID = @CustomerID
    ORDER BY o.OrderDate DESC;
END;
GO

-- 4. Stored Procedure: Get Low Stock Products
IF OBJECT_ID('inventory.sp_GetLowStockProducts', 'P') IS NOT NULL DROP PROCEDURE inventory.sp_GetLowStockProducts;
GO
CREATE PROCEDURE inventory.sp_GetLowStockProducts
    @Threshold INT = NULL
AS
BEGIN
    SET NOCOUNT ON;

    SELECT 
        p.ProductID,
        p.ProductName,
        c.CategoryName,
        p.UnitsInStock,
        p.ReorderLevel,
        s.CompanyName AS SupplierName,
        s.Phone AS SupplierContact
    FROM inventory.Products p
    INNER JOIN inventory.Categories c ON p.CategoryID = c.CategoryID
    LEFT JOIN inventory.Suppliers s ON p.SupplierID = s.SupplierID
    WHERE p.UnitsInStock <= COALESCE(@Threshold, p.ReorderLevel)
      AND p.IsDiscontinued = 0
    ORDER BY p.UnitsInStock ASC;
END;
GO

-- 5. Create a dedicated application test user
USE master;
GO

IF NOT EXISTS (SELECT name FROM sys.server_principals WHERE name = 'dbterm_user')
BEGIN
    CREATE LOGIN dbterm_user WITH PASSWORD = 'DbTermPassword123!', CHECK_POLICY = OFF;
    PRINT 'Login dbterm_user created.';
END
GO

USE SalesDB;
GO

IF NOT EXISTS (SELECT name FROM sys.database_principals WHERE name = 'dbterm_user')
BEGIN
    CREATE USER dbterm_user FOR LOGIN dbterm_user;
    ALTER ROLE db_datareader ADD MEMBER dbterm_user;
    ALTER ROLE db_datawriter ADD MEMBER dbterm_user;
    ALTER ROLE db_ddladmin ADD MEMBER dbterm_user;
    PRINT 'User dbterm_user mapped to SalesDB with read/write/ddl roles.';
END
GO

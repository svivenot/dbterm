-- 02-create-tables.sql
USE SalesDB;
GO

-- Create Custom Schemas
IF NOT EXISTS (SELECT * FROM sys.schemas WHERE name = 'sales')
    EXEC('CREATE SCHEMA sales');
GO

IF NOT EXISTS (SELECT * FROM sys.schemas WHERE name = 'inventory')
    EXEC('CREATE SCHEMA inventory');
GO

IF NOT EXISTS (SELECT * FROM sys.schemas WHERE name = 'hr')
    EXEC('CREATE SCHEMA hr');
GO

IF NOT EXISTS (SELECT * FROM sys.schemas WHERE name = 'audit')
    EXEC('CREATE SCHEMA audit');
GO

-- 1. Departments Table
IF OBJECT_ID('hr.Departments', 'U') IS NOT NULL DROP TABLE hr.Departments;
CREATE TABLE hr.Departments (
    DepartmentID INT IDENTITY(1,1) PRIMARY KEY,
    DepartmentName NVARCHAR(100) NOT NULL,
    Location NVARCHAR(100) NOT NULL DEFAULT 'Paris Headquarter',
    Budget DECIMAL(15,2) NOT NULL DEFAULT 0.00,
    CreatedAt DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
GO

-- 2. Employees Table
IF OBJECT_ID('hr.Employees', 'U') IS NOT NULL DROP TABLE hr.Employees;
CREATE TABLE hr.Employees (
    EmployeeID INT IDENTITY(1,1) PRIMARY KEY,
    FirstName NVARCHAR(50) NOT NULL,
    LastName NVARCHAR(50) NOT NULL,
    Email NVARCHAR(100) UNIQUE NOT NULL,
    JobTitle NVARCHAR(100) NOT NULL,
    DepartmentID INT NULL FOREIGN KEY REFERENCES hr.Departments(DepartmentID),
    HireDate DATE NOT NULL,
    Salary DECIMAL(12,2) NOT NULL,
    IsActive BIT NOT NULL DEFAULT 1,
    Notes NVARCHAR(MAX) NULL
);
GO

-- 3. Customers Table
IF OBJECT_ID('sales.Customers', 'U') IS NOT NULL DROP TABLE sales.Customers;
CREATE TABLE sales.Customers (
    CustomerID NVARCHAR(10) PRIMARY KEY,
    CompanyName NVARCHAR(100) NOT NULL,
    ContactName NVARCHAR(100) NOT NULL,
    ContactTitle NVARCHAR(50) NULL,
    Email NVARCHAR(100) NULL,
    Phone NVARCHAR(30) NULL,
    Address NVARCHAR(200) NULL,
    City NVARCHAR(50) NOT NULL,
    PostalCode NVARCHAR(20) NULL,
    Country NVARCHAR(50) NOT NULL,
    AccountBalance DECIMAL(12,2) NOT NULL DEFAULT 0.00,
    IsVip BIT NOT NULL DEFAULT 0,
    CreatedAt DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
GO

-- 4. Categories Table
IF OBJECT_ID('inventory.Categories', 'U') IS NOT NULL DROP TABLE inventory.Categories;
CREATE TABLE inventory.Categories (
    CategoryID INT IDENTITY(1,1) PRIMARY KEY,
    CategoryName NVARCHAR(50) NOT NULL UNIQUE,
    Description NVARCHAR(255) NULL
);
GO

-- 5. Suppliers Table
IF OBJECT_ID('inventory.Suppliers', 'U') IS NOT NULL DROP TABLE inventory.Suppliers;
CREATE TABLE inventory.Suppliers (
    SupplierID INT IDENTITY(1,1) PRIMARY KEY,
    CompanyName NVARCHAR(100) NOT NULL,
    ContactName NVARCHAR(100) NULL,
    Country NVARCHAR(50) NOT NULL,
    Phone NVARCHAR(30) NULL
);
GO

-- 6. Products Table
IF OBJECT_ID('inventory.Products', 'U') IS NOT NULL DROP TABLE inventory.Products;
CREATE TABLE inventory.Products (
    ProductID INT IDENTITY(1,1) PRIMARY KEY,
    ProductName NVARCHAR(100) NOT NULL,
    CategoryID INT NOT NULL FOREIGN KEY REFERENCES inventory.Categories(CategoryID),
    SupplierID INT NULL FOREIGN KEY REFERENCES inventory.Suppliers(SupplierID),
    UnitPrice DECIMAL(10,2) NOT NULL,
    UnitsInStock INT NOT NULL DEFAULT 0,
    UnitsOnOrder INT NOT NULL DEFAULT 0,
    ReorderLevel INT NOT NULL DEFAULT 10,
    IsDiscontinued BIT NOT NULL DEFAULT 0,
    AttributesJSON NVARCHAR(MAX) NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
GO

-- 7. Orders Table
IF OBJECT_ID('sales.Orders', 'U') IS NOT NULL DROP TABLE sales.Orders;
CREATE TABLE sales.Orders (
    OrderID INT IDENTITY(1000,1) PRIMARY KEY,
    CustomerID NVARCHAR(10) NOT NULL FOREIGN KEY REFERENCES sales.Customers(CustomerID),
    EmployeeID INT NULL FOREIGN KEY REFERENCES hr.Employees(EmployeeID),
    OrderDate DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    RequiredDate DATE NULL,
    ShippedDate DATE NULL,
    Freight DECIMAL(10,2) NOT NULL DEFAULT 0.00,
    ShipCity NVARCHAR(50) NULL,
    ShipCountry NVARCHAR(50) NULL,
    OrderStatus NVARCHAR(20) NOT NULL DEFAULT 'Pending' CHECK (OrderStatus IN ('Pending', 'Processing', 'Shipped', 'Delivered', 'Cancelled'))
);
GO

-- 8. OrderItems Table
IF OBJECT_ID('sales.OrderItems', 'U') IS NOT NULL DROP TABLE sales.OrderItems;
CREATE TABLE sales.OrderItems (
    OrderItemID INT IDENTITY(1,1) PRIMARY KEY,
    OrderID INT NOT NULL FOREIGN KEY REFERENCES sales.Orders(OrderID) ON DELETE CASCADE,
    ProductID INT NOT NULL FOREIGN KEY REFERENCES inventory.Products(ProductID),
    UnitPrice DECIMAL(10,2) NOT NULL,
    Quantity INT NOT NULL CHECK (Quantity > 0),
    Discount DECIMAL(4,2) NOT NULL DEFAULT 0.00
);
GO

-- 9. Audit Logs Table
IF OBJECT_ID('audit.ActivityLogs', 'U') IS NOT NULL DROP TABLE audit.ActivityLogs;
CREATE TABLE audit.ActivityLogs (
    LogID BIGINT IDENTITY(1,1) PRIMARY KEY,
    EventType NVARCHAR(50) NOT NULL,
    TableName NVARCHAR(50) NOT NULL,
    RecordKey NVARCHAR(100) NOT NULL,
    ExecutedBy NVARCHAR(100) NOT NULL DEFAULT SUSER_SNAME(),
    LogTimestamp DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    Details NVARCHAR(MAX) NULL
);
GO

-- Indexes for performance
CREATE INDEX IX_Orders_CustomerID ON sales.Orders(CustomerID);
CREATE INDEX IX_Orders_OrderDate ON sales.Orders(OrderDate);
CREATE INDEX IX_OrderItems_OrderID ON sales.OrderItems(OrderID);
CREATE INDEX IX_OrderItems_ProductID ON sales.OrderItems(ProductID);
CREATE INDEX IX_Products_CategoryID ON inventory.Products(CategoryID);
GO

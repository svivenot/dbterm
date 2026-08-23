-- 01-create-database.sql
-- Create databases for testing dbterm

IF NOT EXISTS (SELECT name FROM sys.databases WHERE name = N'SalesDB')
BEGIN
    CREATE DATABASE SalesDB;
    PRINT 'Database SalesDB created.';
END
GO

IF NOT EXISTS (SELECT name FROM sys.databases WHERE name = N'NorthwindTui')
BEGIN
    CREATE DATABASE NorthwindTui;
    PRINT 'Database NorthwindTui created.';
END
GO

-- 03-seed-data.sql
USE SalesDB;
GO

SET NOCOUNT ON;

-- 1. Populate Departments
INSERT INTO hr.Departments (DepartmentName, Location, Budget) VALUES
('Executive Management', 'Paris - 8ème', 1250000.00),
('Information Technology', 'Paris - La Défense', 3500000.00),
('Sales & Partnerships', 'Lyon', 2100000.00),
('Customer Support', 'Nantes', 850000.00),
('Finance & Legal', 'Paris - 8ème', 980000.00);
GO

-- 2. Populate Employees
INSERT INTO hr.Employees (FirstName, LastName, Email, JobTitle, DepartmentID, HireDate, Salary, IsActive, Notes) VALUES
('Sylvain', 'Dupont', 'sylvain.dupont@company.com', 'Chief Technology Officer', 2, '2019-03-15', 115000.00, 1, 'Key architect for DB modernization'),
('Claire', 'Lefebvre', 'claire.lefebvre@company.com', 'Lead Database Administrator', 2, '2020-06-01', 82000.00, 1, 'Expert in MSSQL and Oracle tuning'),
('Thomas', 'Martin', 'thomas.martin@company.com', 'Senior Backend Engineer', 2, '2021-01-10', 74000.00, 1, 'Go & Distributed systems specialist'),
('Camille', 'Bernard', 'camille.bernard@company.com', 'VP Sales', 3, '2018-09-01', 95000.00, 1, 'Enterprise accounts manager'),
('Antoine', 'Moreau', 'antoine.moreau@company.com', 'Key Account Executive', 3, '2022-04-15', 58000.00, 1, 'EMEA region'),
('Sophie', 'Roux', 'sophie.roux@company.com', 'Financial Controller', 5, '2021-11-20', 65000.00, 1, 'Quarterly audit and reporting');
GO

-- 3. Populate Customers
INSERT INTO sales.Customers (CustomerID, CompanyName, ContactName, ContactTitle, Email, Phone, Address, City, PostalCode, Country, AccountBalance, IsVip) VALUES
('CUST001', 'Airbus Group SAS', 'Jean-Luc Picard', 'Procurement Director', 'jl.picard@airbus.example.com', '+33 5 61 93 33 33', '1 Rond-Point Maurice Bellonte', 'Toulouse', '31707', 'France', 45000.00, 1),
('CUST002', 'Siemens AG', 'Klaus Weber', 'Head of IT Infrastructure', 'k.weber@siemens.example.com', '+49 89 636 00', 'Werner-von-Siemens-Str. 1', 'Munich', '80333', 'Germany', 125000.00, 1),
('CUST003', 'TotalEnergies SE', 'Valérie Bertrand', 'Data Operations Lead', 'v.bertrand@total.example.com', '+33 1 47 44 45 46', '2 Place Jean Millier', 'Courbevoie', '92078', 'France', 8900.00, 0),
('CUST004', 'Novartis Pharma', 'Beatrix Müller', 'Systems Architect', 'b.mueller@novartis.example.com', '+41 61 324 11 11', 'Lichtstrasse 35', 'Basel', '4056', 'Switzerland', 62000.00, 1),
('CUST005', 'Barclays Bank PLC', 'Arthur Pendelton', 'VP Cloud Platform', 'a.pendelton@barclays.example.com', '+44 20 7116 1000', '1 Churchill Place', 'London', 'E14 5HP', 'United Kingdom', 210500.00, 1),
('CUST006', 'Spotify AB', 'Linus Torvaldsen', 'Infrastructure SRE', 'linus@spotify.example.com', '+46 8 500 000 00', 'Birger Jarlsgatan 61', 'Stockholm', '11356', 'Sweden', 3400.00, 0),
('CUST007', 'Dassault Systèmes', 'Élise Fontaine', 'R&D Director', 'elise.fontaine@3ds.example.com', '+33 1 61 62 61 62', '10 Rue Marcel Dassault', 'Vélizy-Villacoublay', '78140', 'France', 78900.00, 1),
('CUST008', 'Santander Group', 'Mateo Fernandez', 'Database Lead', 'm.fernandez@santander.example.com', '+34 91 289 00 00', 'Paseo de Pereda 9', 'Santander', '39004', 'Spain', 15400.00, 0);
GO

-- 4. Populate Categories
INSERT INTO inventory.Categories (CategoryName, Description) VALUES
('Enterprise Servers', 'Rack-mounted compute nodes and blade enclosures'),
('Storage & SAN', 'NVMe All-Flash arrays and high-capacity backup drives'),
('Networking', 'Core switches, 100GbE routers, and firewall appliances'),
('Cloud Licenses', 'Managed database clusters and enterprise software licenses'),
('Developer Workstations', 'High-end developer laptops and multi-monitor setups');
GO

-- 5. Populate Suppliers
INSERT INTO inventory.Suppliers (CompanyName, ContactName, Country, Phone) VALUES
('Dell Technologies', 'Robert Miller', 'United States', '+1 800 456 3355'),
('Cisco Systems EMEA', 'Maria Rossi', 'Netherlands', '+31 20 357 1000'),
('OVHcloud Group', 'Octave Klaba', 'France', '+33 9 72 10 10 07'),
('Lenovo Enterprise', 'Chen Wei', 'Hong Kong', '+852 2516 3838');
GO

-- 6. Populate Products
INSERT INTO inventory.Products (ProductName, CategoryID, SupplierID, UnitPrice, UnitsInStock, UnitsOnOrder, ReorderLevel, IsDiscontinued, AttributesJSON) VALUES
('PowerEdge R760 Rack Server', 1, 1, 6499.00, 15, 5, 3, 0, '{"cpu":"Dual Xeon Platinum 8480+","ram_gb":512,"storage":"8x 3.84TB NVMe"}'),
('PowerEdge R660 1U Server', 1, 1, 4199.00, 24, 0, 5, 0, '{"cpu":"Xeon Gold 6448Y","ram_gb":256,"storage":"4x 1.92TB NVMe"}'),
('PowerStore 5000T Flash Array', 2, 1, 28500.00, 4, 2, 1, 0, '{"raw_tb":96,"effective_tb":380,"protocols":["iSCSI","FC","NVMe-oF"]}'),
('Cisco Catalyst 9500 40-Port', 3, 2, 14200.00, 8, 4, 2, 0, '{"ports":40,"speed":"100GbE","stackable":true}'),
('Cisco Firepower 2140 Appliance', 3, 2, 8900.00, 12, 0, 3, 0, '{"throughput_gbps":8.5,"vpn_support":true}'),
('OVH Hosted MSSQL Dedicated Node', 4, 3, 850.00, 100, 0, 10, 0, '{"cores":16,"ram_gb":64,"managed_backup":true}'),
('PostgreSQL HA Multi-AZ License', 4, 3, 490.00, 200, 0, 20, 0, '{"sla":"99.99%","replication":"synchronous"}'),
('ThinkPad P1 Gen 6 Mobile Workstation', 5, 4, 3200.00, 35, 10, 5, 0, '{"cpu":"i9-13900H","gpu":"RTX 4080","ram_gb":64,"screen":"16-inch 4K OLED"}'),
('ThinkVision 31.5-inch 4K Monitor', 5, 4, 750.00, 60, 20, 10, 0, '{"resolution":"3840x2160","panel":"IPS","usb_c_hub":true}');
GO

-- 7. Populate Orders
INSERT INTO sales.Orders (CustomerID, EmployeeID, OrderDate, RequiredDate, ShippedDate, Freight, ShipCity, ShipCountry, OrderStatus) VALUES
('CUST001', 4, DATEADD(day, -45, SYSUTCDATETIME()), DATEADD(day, -30, SYSUTCDATETIME()), DATEADD(day, -32, SYSUTCDATETIME()), 450.00, 'Toulouse', 'France', 'Delivered'),
('CUST002', 4, DATEADD(day, -30, SYSUTCDATETIME()), DATEADD(day, -15, SYSUTCDATETIME()), DATEADD(day, -16, SYSUTCDATETIME()), 1200.00, 'Munich', 'Germany', 'Delivered'),
('CUST003', 5, DATEADD(day, -20, SYSUTCDATETIME()), DATEADD(day, -5, SYSUTCDATETIME()), DATEADD(day, -6, SYSUTCDATETIME()), 320.00, 'Courbevoie', 'France', 'Delivered'),
('CUST004', 5, DATEADD(day, -10, SYSUTCDATETIME()), DATEADD(day, 5, SYSUTCDATETIME()), DATEADD(day, -2, SYSUTCDATETIME()), 800.00, 'Basel', 'Switzerland', 'Shipped'),
('CUST005', 4, DATEADD(day, -5, SYSUTCDATETIME()), DATEADD(day, 10, SYSUTCDATETIME()), NULL, 1500.00, 'London', 'United Kingdom', 'Processing'),
('CUST007', 5, DATEADD(day, -2, SYSUTCDATETIME()), DATEADD(day, 14, SYSUTCDATETIME()), NULL, 280.00, 'Vélizy-Villacoublay', 'France', 'Pending');
GO

-- 8. Populate OrderItems
INSERT INTO sales.OrderItems (OrderID, ProductID, UnitPrice, Quantity, Discount) VALUES
(1000, 1, 6499.00, 4, 0.05),
(1000, 4, 14200.00, 2, 0.10),
(1001, 3, 28500.00, 2, 0.00),
(1001, 2, 4199.00, 6, 0.05),
(1001, 5, 8900.00, 2, 0.00),
(1002, 8, 3200.00, 5, 0.00),
(1002, 9, 750.00, 10, 0.05),
(1003, 1, 6499.00, 8, 0.08),
(1003, 6, 850.00, 12, 0.00),
(1004, 3, 28500.00, 4, 0.12),
(1004, 4, 14200.00, 4, 0.05),
(1005, 8, 3200.00, 10, 0.05);
GO

-- 9. Populate Audit Logs
INSERT INTO audit.ActivityLogs (EventType, TableName, RecordKey, Details) VALUES
('INSERT', 'sales.Customers', 'CUST001', 'New customer onboarded: Airbus Group'),
('UPDATE', 'inventory.Products', '1', 'Price adjusted from 6200.00 to 6499.00'),
('INSERT', 'sales.Orders', '1000', 'Order placed by Airbus Group for 4 servers');
GO

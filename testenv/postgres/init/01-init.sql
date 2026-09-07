-- PostgreSQL Initialization Script for dbterm test environment
CREATE SCHEMA IF NOT EXISTS sales;
CREATE SCHEMA IF NOT EXISTS inventory;
CREATE SCHEMA IF NOT EXISTS hr;

-- HR Tables
CREATE TABLE IF NOT EXISTS hr.departments (
    department_id SERIAL PRIMARY KEY,
    department_name VARCHAR(100) NOT NULL,
    location VARCHAR(100) NOT NULL DEFAULT 'Paris Headquarter',
    budget NUMERIC(15,2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS hr.employees (
    employee_id SERIAL PRIMARY KEY,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    job_title VARCHAR(100) NOT NULL,
    department_id INT REFERENCES hr.departments(department_id),
    hire_date DATE NOT NULL,
    salary NUMERIC(12,2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    notes TEXT
);

-- Customers Table
CREATE TABLE IF NOT EXISTS sales.customers (
    customer_id VARCHAR(10) PRIMARY KEY,
    company_name VARCHAR(100) NOT NULL,
    contact_name VARCHAR(100) NOT NULL,
    contact_title VARCHAR(50),
    email VARCHAR(100),
    phone VARCHAR(30),
    city VARCHAR(50) NOT NULL,
    country VARCHAR(50) NOT NULL,
    account_balance NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    is_vip BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Inventory Tables
CREATE TABLE IF NOT EXISTS inventory.categories (
    category_id SERIAL PRIMARY KEY,
    category_name VARCHAR(50) NOT NULL UNIQUE,
    description VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS inventory.products (
    product_id SERIAL PRIMARY KEY,
    product_name VARCHAR(100) NOT NULL,
    category_id INT NOT NULL REFERENCES inventory.categories(category_id),
    unit_price NUMERIC(10,2) NOT NULL,
    units_in_stock INT NOT NULL DEFAULT 0,
    units_on_order INT NOT NULL DEFAULT 0,
    reorder_level INT NOT NULL DEFAULT 10,
    is_discontinued BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Orders Tables
CREATE TABLE IF NOT EXISTS sales.orders (
    order_id SERIAL PRIMARY KEY,
    customer_id VARCHAR(10) NOT NULL REFERENCES sales.customers(customer_id),
    employee_id INT REFERENCES hr.employees(employee_id),
    order_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    freight NUMERIC(10,2) NOT NULL DEFAULT 0.00,
    ship_city VARCHAR(50),
    ship_country VARCHAR(50),
    order_status VARCHAR(20) NOT NULL DEFAULT 'Pending'
);

CREATE TABLE IF NOT EXISTS sales.order_items (
    order_item_id SERIAL PRIMARY KEY,
    order_id INT NOT NULL REFERENCES sales.orders(order_id) ON DELETE CASCADE,
    product_id INT NOT NULL REFERENCES inventory.products(product_id),
    unit_price NUMERIC(10,2) NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    discount NUMERIC(4,2) NOT NULL DEFAULT 0.00
);

-- Views
CREATE OR REPLACE VIEW sales.v_order_summary AS
SELECT 
    o.order_id,
    o.order_date,
    o.order_status,
    c.customer_id,
    c.company_name,
    c.country,
    CONCAT(e.first_name, ' ', e.last_name) AS sales_rep,
    COUNT(oi.order_item_id) AS item_count,
    COALESCE(SUM(oi.unit_price * oi.quantity * (1.0 - oi.discount)), 0) AS sub_total,
    o.freight,
    COALESCE(SUM(oi.unit_price * oi.quantity * (1.0 - oi.discount)), 0) + o.freight AS grand_total
FROM sales.orders o
INNER JOIN sales.customers c ON o.customer_id = c.customer_id
LEFT JOIN hr.employees e ON o.employee_id = e.employee_id
LEFT JOIN sales.order_items oi ON o.order_id = oi.order_id
GROUP BY 
    o.order_id, o.order_date, o.order_status, c.customer_id, c.company_name, c.country,
    e.first_name, e.last_name, o.freight;

-- Seed Data
INSERT INTO hr.departments (department_name, location, budget) VALUES
('Executive Management', 'Paris - 8ème', 1250000.00),
('Information Technology', 'Paris - La Défense', 3500000.00),
('Sales & Partnerships', 'Lyon', 2100000.00);

INSERT INTO sales.customers (customer_id, company_name, contact_name, email, city, country, account_balance, is_vip) VALUES
('CUST001', 'Airbus Group SAS', 'Jean-Luc Picard', 'jl.picard@airbus.example.com', 'Toulouse', 'France', 45000.00, TRUE),
('CUST002', 'Siemens AG', 'Klaus Weber', 'k.weber@siemens.example.com', 'Munich', 'Germany', 125000.00, TRUE),
('CUST003', 'TotalEnergies SE', 'Valérie Bertrand', 'v.bertrand@total.example.com', 'Courbevoie', 'France', 8900.00, FALSE);

INSERT INTO inventory.categories (category_name, description) VALUES
('Enterprise Servers', 'Rack-mounted compute nodes and blade enclosures'),
('Storage & SAN', 'NVMe All-Flash arrays and high-capacity backup drives');

INSERT INTO inventory.products (product_name, category_id, unit_price, units_in_stock) VALUES
('PowerEdge R760 Rack Server', 1, 6499.00, 15),
('PowerEdge R660 1U Server', 1, 4199.00, 24),
('PowerStore 5000T Flash Array', 2, 28500.00, 4);

-- Use the application database

-- Create a user mapped to the login
CREATE USER [orcauser] FOR LOGIN [orcalogin];

-- Grant access to the "orca" database
ALTER ROLE db_datareader ADD MEMBER [orcauser];
ALTER ROLE db_datawriter ADD MEMBER [orcauser];

-- Create the session table
CREATE TABLE dbo.Session
(
    SessionId INT PRIMARY KEY,
    UserId INT,
    StartTime DATETIME,
    EndTime DATETIME
);
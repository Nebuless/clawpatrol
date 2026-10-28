CREATE TABLE support_control (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 enabled INTEGER NOT NULL CHECK(enabled IN (0,1))
);
INSERT INTO support_control VALUES(1,1);

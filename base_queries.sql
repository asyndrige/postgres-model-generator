SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE';

SELECT 
  c.table_name, c.column_name, c.ordinal_position, c.column_default, c.is_nullable, c.data_type, c.udt_name, 
  c.character_maximum_length, c.character_octet_length, c.numeric_precision
FROM 
	information_schema.columns AS c 
JOIN
	information_schema.tables as t
ON
	t.table_name = c.table_name
WHERE 
  t.table_schema = 'public' AND t.table_type = 'BASE TABLE'
ORDER BY 
  c.table_name;

# feat - Search
Currently, VSS only allow you to get variables by exact name or by using a wildcard ('*') to match multiple variables. However, there are scenarios where more advanced search capabilities would be beneficial, such as searching by metadata, value patterns, or other criteria.

## proposal
add the method 'search' to the controller and APIS to allow clients to perform advanced searches on the stored variables. The search method should support various criteria, such as:
- Metadata search: Allow clients to search for variables based on their associated metadata.
- Value pattern search: Allow clients to search for variables whose values match a specific pattern (e.g., regex).
- Range search: For numeric variables, allow clients to search for variables within a specific range.
- Key: search for keys

The search also could contains a 'context', e.g., a parent variable where search should be performed.

Also, add a device (interceptor?) to index data variables
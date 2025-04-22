Consider the first example in Figure 1 of MVS. I want to add this example as an integration test.

Specifically, I don't want you to mock data in a custom way but rather that you use the CLI commands 
* "cosm init ..."
* "cosm add ..."
* "cosm registry init ..."
* "cosm registry add ..."
* "cosm release ..."
* "cosm rm ..."
* "cosm activate"
Even better is that you call these commands from helper methods in the test file.

Let me summarize the diagram for you, such that we are in the same page:
These are all the packages:
* A1
* B1.1, B1.2
* C1.1, C1.2, C.1.3
* D1.1, D1.2, D.1.3, D1.4
* E1.1, E1.2, E1.3
* F1.1
* G1.1

The direct dependencies of each of the packages are (in the first example that we are looking at now):
* A1 depends on B1.2 and C1.2
* B1.1 depends on D1.1
* B1.2 depends on D1.3
* C1.2 depends on D1.4
* C1.3 depends on F.1.1
* D1.1 depends on E1.1
* D1.2 depends on E1.1
* D1.3 depends on E1.2
* D1.4 depends on E1.2
* F1.1 depends on G1.1
* G1.1 depends on F1.1

The objective now is to test all integrated commands and obtain the correct build list after calling "cosm activate"

Please construct the example by keeping the following in mind:
* Use "cosm init <package name>" to initialize a new package
* Add it to a registry using "cosm registry add ..."
* You can only initialize and add a package to a registry once. Different versions can be instantiated and released to the registry using "cosm release ..."
# Autz: A simple Authorization package based on the Google IAM Policy framework

Creating a simple Authorization package based on the Google IAM (Identity and Access Management) Policy framework involves defining and managing access controls for resources.

This package does not handle the identification / authn part of the IAM framework. It only deals with the Authorisation / Authz side of the framework. It authorises whether a particular **Principal** (user or service account) is able to perform a particular **Permission** (Get, Update, List, etc.) on a particular **Resource**. A resource is defined in the context of a Resource Driven development framework as defined at [RDD](https://google.com) inline with the [API Improvement Proposals](https://aip.dev)

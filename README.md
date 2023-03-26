# Enket
[![Go Report Card](https://goreportcard.com/badge/github.com/georgesolomos/enket)](https://goreportcard.com/report/github.com/georgesolomos/enket)


**Find the cheapest electricity provider for you**

This tool reads your electricity usage data, in the form of a CSV file adhering to the
Australian Energy Market Operator (AEMO) Meter Data File Format (MDFF) specification, containing
NEM12 formatted data. It reads in both energy imports and solar exports.

It then calculates what your electricity costs would be with all the energy retailers in Australia,
using up-to-date data provided by the Government's Consumer Data Standards API.

This allows you to find the lowest price for your specific electricity usage.

## Resources
- [AEMO metering procedures, guidelines and processes](https://aemo.com.au/energy-systems/electricity/national-electricity-market-nem/market-operations/retail-and-metering/metering-procedures-guidelines-and-processes) - Contains up-to-date documentation including the specification for the MDFF format.
- [How to access your smart meter data](https://support.solarquotes.com.au/hc/en-us/articles/360001312176-How-to-access-your-smart-meter-data-)
- [Consumer Data Standards](https://consumerdatastandardsaustralia.github.io/standards)
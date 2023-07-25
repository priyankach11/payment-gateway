package main

import (
  "context"
  "flag"
  "fmt"
  "log"

  "cloud.google.com/go/logging"
  "cloud.google.com/go/logging/logadmin"
  "google.golang.org/api/option"
  "google.golang.org/api/option/internaloption"
  pb "google.golang.org/genproto/googleapis/cloud/paymentgateway/issuerswitch/v1"
  "google.golang.org/grpc"
)

var (
  txnid = flag.String("txnid", "", "Transaction ID of the request for which logs are to be displayed")
  rrn   = flag.String("rrn", "", "RRN of the payment for which logs are to be displayed")
  addr  = flag.String("addr", "", "API Service external endpoint")
)

func generateAPIFilter(rrn string, tld []*txnLogDetails) []string {
  var filterList []string
  for _, d := range tld {
   id := d.getTxnId()
   filterList = append(filterList, fmt.Sprintf("transactionID = %s", id))
  }
  if filterList == nil {
   filterList = append(filterList, fmt.Sprintf("rrn = %s", rrn))
  }
  return filterList
}

func validateParams(rrn, txnid string) error {
  if rrn == "" && txnid == "" {
   return fmt.Errorf("User should provide atleast one of rrn or txnid")
  }
  return nil
}

func main() {
  flag.Parse()
  ctx := context.Background()

  err := validateParams(*rrn, *txnid)
  if err != nil {
   log.Fatalf("Invalid argument: %v", err)
  }
  opts := []option.ClientOption{
   option.WithScopes(logging.ReadScope),
   internaloption.WithDefaultEndpoint("staging-logging.sandbox.googleapis.com:443"),
  }
  logClient, err := newLogClient(ctx, opts...)
  if err != nil {
   log.Print(err)
  }
  var filterLog logadmin.EntriesOption
  if *txnid != "" {
   filterLog = logadmin.Filter(getFilterStringIDs(*txnid))
  } else {
   filterString := fmt.Sprintf("\"custRef=\\\"%s\\\"\"", *rrn)
   filterLog = logadmin.Filter(getFilterStringIDs(filterString))
  }

  txnLogDetails, err := logClient.getTxnIDs(ctx, filterLog)
  if err != nil {
   log.Print(err)
  }

  for _, d := range txnLogDetails {
   filterLogs := logadmin.Filter(getFilterString(d.getTxnId(), d.getMsgId()))
   logs, err := logClient.getLogs(ctx, filterLogs)
   if err != nil {
    log.Print(err)
   }
   printTxnDetails(d)
   baLog, _ := processAndPrintLogs(logs)
   if baLog != "" {
    reqID := logClient.requestID(baLog)
    fmt.Println("request ID is", reqID)
   }
  }

  conn, err := grpc.Dial(*addr, grpc.WithInsecure())
  if err != nil {
   log.Fatalf("did not connect: %v", err)
  }
  defer conn.Close()
  client := pb.NewIssuerSwitchTransactionsClient(conn)
  reqFilter := generateAPIFilter(*rrn, txnLogDetails)
  err = printListTransactionsResponse(ctx, client, reqFilter)
}
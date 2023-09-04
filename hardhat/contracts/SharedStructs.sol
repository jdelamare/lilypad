// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.6;

library SharedStructs {

  enum ServiceType {
    Solver,
    Mediator,
    Directory,
    ResourceProvider,
    JobCreator
  }

  enum AgreementState {

    // the two parties have not yet both agreed to the deal
    DealNegotiating,

    // both parties have agreed
    DealAgreed,

    // results have been submitted by the RP
    ResultsSubmitted,

    // the JC has accepted the results
    ResultsAccepted,

    // the JC has checked the results
    ResultsChecked,

    // a mediator has accepted the results
    MediationAccepted,

    // a mediator has rejected the results
    MediationRejected,

    // this means the RP did not submit results in time
    TimeoutSubmitResults,

    // this means the JC did not accept or reject results in time
    TimeoutJudgeResults,

    // this means the mediator did not accept or submit judgement in time
    TimeoutMediateResults

  }

  // we map addresses onto infomation about the user
  struct User {
    address userAddress;
    // the CID of information for this user
    uint256 metadataCID;
    string url;
    ServiceType[] roles;
    address[] trustedMediators;
    address[] trustedDirectories;
  }

  // a Deal forms the information that is agreed between both parties
  // both parties must have called "agree_deal" with the exact
  // same parameters before the deal is considered valid
  // a Deal is immutable - nothing about it can be updated
  struct Deal {
    // the CID of the Deal document on IPFS (and directory service)
    // this contains the job spec, the job offer and the resource offer
    uint256 dealId;

    // the addresses of the RP and JC that have agreed to this deal
    address resourceProvider;
    address jobCreator;

    // agreed price per instruction
    uint256 instructionPrice;

    // the max time we will wait for the results
    uint256 timeout;

    // the collateral that the RP has put up to prevent timeouts
    uint256 timeoutCollateral;

    // the collateral that the JC has put up to pay for the job
    // the final cost of the job will be deducted from this
    uint256 paymentCollateral;

    // how much collateral the RP will post to attest it's results are correct
    // this is a multiple of the cost of the job which is known at results
    // submission time
    uint256 resultsCollateralMultiple;

    // how much is the JC willing to pay the mediator to resolve disputes
    uint256 mediationFee;
  }

  // what the RP submits back once having run the job
  // this is also immutable
  struct Result {
    // the id of the deal that this result is for
    uint256 dealId;

    // the CID of the results on IPFS (and directory service)
    uint256 resultsId;

    // how many instructions were executed by the RP
    uint256 instructionCount;
  }

  // an agreement keeps track of the state of a deal and it's fields can be mutated
  struct Agreement {
    // the current state of the agreement
    AgreementState state;

    // if the JC has checked the result
    // what mediator did they choose?
    // (this must be in the list of the RP and JC's trusted mediators)
    address mediator;

    uint256 resourceProviderAgreedAt;
    uint256 jobCreatorAgreedAt;
    uint256 dealAgreedAt;

    uint256 resultsSubmittedAt;
    uint256 resultsAcceptedAt;
    uint256 resultsCheckedAt;

    uint256 mediationAcceptedAt;
    uint256 mediationRejectedAt;

    uint256 timeoutSubmitResultsAt;
    uint256 timeoutJudgeResultsAt;
    uint256 timeoutMediateResultsAt;
  }
}

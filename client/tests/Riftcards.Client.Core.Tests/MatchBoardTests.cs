using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;
using Xunit;

namespace Riftcards.Client.Core.Tests;

public sealed class MatchBoardTests
{
    [Fact]
    public void SelectsUnitCardAndSubmitsImmediately()
    {
        var board = new MatchBoard();
        board.Apply(View(hand: [Card("c1", ProtocolNames.CardKindUnit)]));

        var intent = board.SelectHandCard("c1");

        var value = Assert.IsType<CommandIntent>(intent);
        Assert.Equal(ProtocolNames.PlayCard, value.Type);
        Assert.Equal("c1", value.CardInstanceId);
        Assert.Null(value.TargetId);
        Assert.Equal(SelectionKind.None, board.Selection);
    }

    [Fact]
    public void SpellWaitsForTargetThenSubmitsWithTarget()
    {
        var board = new MatchBoard();
        board.Apply(View(hand: [Card("bolt", ProtocolNames.CardKindSpell, "arcane_bolt", 2)]));

        Assert.Null(board.SelectHandCard("bolt"));
        Assert.Equal(SelectionKind.Card, board.Selection);
        Assert.True(board.AwaitingTarget);

        var intent = board.SelectTarget("hero-1");

        var value = Assert.IsType<CommandIntent>(intent);
        Assert.Equal(ProtocolNames.PlayCard, value.Type);
        Assert.Equal("bolt", value.CardInstanceId);
        Assert.Equal("hero-1", value.TargetId);
        Assert.False(board.AwaitingTarget);
    }

    [Fact]
    public void TargetWithoutSelectedCardProducesNoIntent()
    {
        var board = new MatchBoard();
        board.Apply(View(hand: [Card("bolt", ProtocolNames.CardKindSpell)]));

        Assert.Null(board.SelectTarget("hero-1"));
    }

    [Fact]
    public void IgnoresInteractionWhenNotYourTurn()
    {
        var board = new MatchBoard();
        board.Apply(View(activeSeat: 1, hand: [Card("c1", ProtocolNames.CardKindUnit)]));

        Assert.False(board.IsYourTurn);
        Assert.Null(board.SelectHandCard("c1"));
        Assert.Null(board.EndTurn());
        Assert.Equal(SelectionKind.None, board.Selection);
    }

    [Fact]
    public void AttackFlowSelectsOwnUnitThenEnemyUnit()
    {
        var board = new MatchBoard();
        board.Apply(View(
            yourBoard: [Unit("mine")],
            opponentBoard: [Unit("theirs")]));

        Assert.Null(board.SelectBoardUnit("mine"));
        Assert.Equal(SelectionKind.Attacker, board.Selection);

        var intent = board.SelectBoardUnit("theirs");

        var value = Assert.IsType<CommandIntent>(intent);
        Assert.Equal(ProtocolNames.Attack, value.Type);
        Assert.Equal("mine", value.CardInstanceId);
        Assert.Equal("theirs", value.TargetId);
        Assert.Equal(SelectionKind.None, board.Selection);
    }

    [Fact]
    public void AttacksOpponentHeroWithProjectedHeroId()
    {
        var board = new MatchBoard();
        board.Apply(View(youSeat: 1, activeSeat: 1, yourBoard: [Unit("mine")]));

        Assert.Null(board.SelectBoardUnit("mine"));
        var intent = board.SelectHero(0);

        var value = Assert.IsType<CommandIntent>(intent);
        Assert.Equal(ProtocolNames.Attack, value.Type);
        Assert.Equal("hero-0", value.TargetId);
    }

    [Fact]
    public void CannotTargetOwnHero()
    {
        var board = new MatchBoard();
        board.Apply(View(youSeat: 0, yourBoard: [Unit("mine")]));

        Assert.Null(board.SelectBoardUnit("mine"));
        Assert.Null(board.SelectHero(0));
        Assert.Equal(SelectionKind.Attacker, board.Selection);
    }

    [Fact]
    public void ClearsSelectionWhenSnapshotNoLongerContainsCard()
    {
        var board = new MatchBoard();
        board.Apply(View(hand: [Card("bolt", ProtocolNames.CardKindSpell)]));
        Assert.Null(board.SelectHandCard("bolt"));
        Assert.Equal(SelectionKind.Card, board.Selection);

        board.Apply(View(hand: [Card("other", ProtocolNames.CardKindUnit)]));

        Assert.Equal(SelectionKind.None, board.Selection);
        Assert.Null(board.SelectedCardInstanceId);
    }

    [Fact]
    public void ClearsSelectionWhenTurnEnds()
    {
        var board = new MatchBoard();
        board.Apply(View(hand: [Card("bolt", ProtocolNames.CardKindSpell)]));
        Assert.Null(board.SelectHandCard("bolt"));

        board.Apply(View(activeSeat: 1, hand: [Card("bolt", ProtocolNames.CardKindSpell)]));

        Assert.Equal(SelectionKind.None, board.Selection);
    }

    [Fact]
    public void ClearsSelectionAndBlocksActionsWhenFinished()
    {
        var board = new MatchBoard();
        board.Apply(View(hand: [Card("bolt", ProtocolNames.CardKindSpell)]));
        Assert.Null(board.SelectHandCard("bolt"));

        board.Apply(View(status: "finished", hand: [Card("bolt", ProtocolNames.CardKindSpell)]));

        Assert.True(board.IsFinished);
        Assert.Equal(SelectionKind.None, board.Selection);
        Assert.Null(board.EndTurn());
        Assert.Null(board.Surrender());
    }

    [Fact]
    public void SurrenderIsAvailableOnOpponentTurn()
    {
        var board = new MatchBoard();
        board.Apply(View(activeSeat: 1));

        var intent = board.Surrender();

        var value = Assert.IsType<CommandIntent>(intent);
        Assert.Equal(ProtocolNames.Surrender, value.Type);
    }

    [Fact]
    public void OpponentHandContentsAreNotExposed()
    {
        var board = new MatchBoard();
        board.Apply(View(opponentHandCount: 5));

        var opponent = Assert.IsType<PlayerView>(board.Opponent);
        Assert.Empty(opponent.Hand);
        Assert.Equal(5, opponent.HandCount);
        Assert.Equal(21, opponent.DeckCount);
    }

    [Fact]
    public void NoInteractionWithoutMatch()
    {
        var board = new MatchBoard();
        board.Apply(null);

        Assert.False(board.HasMatch);
        Assert.False(board.IsYourTurn);
        Assert.Null(board.SelectHandCard("c1"));
        Assert.Null(board.SelectBoardUnit("u1"));
        Assert.Null(board.SelectHero(1));
        Assert.Null(board.EndTurn());
        Assert.Null(board.Surrender());
    }

    private static MatchView View(
        int youSeat = 0,
        int activeSeat = 0,
        string status = "active",
        IEnumerable<CardView>? hand = null,
        IEnumerable<UnitView>? yourBoard = null,
        IEnumerable<UnitView>? opponentBoard = null,
        int opponentHandCount = 4)
    {
        var opponentSeat = 1 - youSeat;
        return new MatchView
        {
            MatchId = "m_test",
            Revision = 3,
            LastEventSeq = 5,
            Turn = 2,
            ActiveSeat = activeSeat,
            Status = status,
            WinnerSeat = null,
            YouSeat = youSeat,
            Players =
            [
                new PlayerView
                {
                    Seat = youSeat,
                    HeroId = $"hero-{youSeat}",
                    Hp = 30,
                    Energy = 3,
                    TurnNumber = 2,
                    DeckCount = 20,
                    HandCount = hand?.Count() ?? 0,
                    Hand = hand?.ToList() ?? [],
                    Board = yourBoard?.ToList() ?? [],
                },
                new PlayerView
                {
                    Seat = opponentSeat,
                    HeroId = $"hero-{opponentSeat}",
                    Hp = 27,
                    Energy = 0,
                    TurnNumber = 1,
                    DeckCount = 21,
                    HandCount = opponentHandCount,
                    Hand = [],
                    Board = opponentBoard?.ToList() ?? [],
                },
            ],
        };
    }

    private static CardView Card(string instanceId, string kind, string cardId = "ember_squire", int cost = 1) =>
        new()
        {
            InstanceId = instanceId,
            CardId = cardId,
            Name = cardId,
            Cost = cost,
            Kind = kind,
        };

    private static UnitView Unit(string instanceId, bool guard = false) =>
        new()
        {
            InstanceId = instanceId,
            CardId = "raider",
            Name = "raider",
            Attack = 4,
            Health = 2,
            MaxHealth = 2,
            Guard = guard,
            AttacksRemaining = 1,
        };
}

using Godot;

namespace Riftcards.Client.Godot;

/// <summary>
/// 界面视觉约定的唯一位置（见 ai-docs/architecture/client-ui.md）：
/// 颜色、字号、间距与样式工厂。界面是代码构建的，因此约定集中在这里，将来换主题资源只改一处。
/// </summary>
public static class UiTheme
{
    public static readonly Color Background = Color.FromHtml("151821");
    public static readonly Color Panel = Color.FromHtml("1f2430");
    public static readonly Color PanelBorder = Color.FromHtml("333a4a");
    public static readonly Color TextPrimary = Color.FromHtml("e6e9f0");
    public static readonly Color TextSecondary = Color.FromHtml("9aa4b8");
    public static readonly Color TextMuted = Color.FromHtml("6b7385");
    public static readonly Color Accent = Color.FromHtml("4ade80");
    public static readonly Color Warning = Color.FromHtml("f0a23c");
    public static readonly Color Danger = Color.FromHtml("ef5350");
    public static readonly Color Guard = Color.FromHtml("5b8def");
    public static readonly Color Selection = Color.FromHtml("f5c451");

    public const int FontTitle = 20;
    public const int FontPanelTitle = 15;
    public const int FontBody = 14;
    public const int FontCaption = 12;

    public const int PanelPadding = 10;
    public const int ElementSeparation = 6;
    public const int SectionSeparation = 8;
    public const int CornerRadius = 6;
    public const int EventPanelHeight = 96;
    public const int CardWidth = 132;
    public const int CardHeight = 52;
    public const int UnitWidth = 124;
    public const int UnitHeight = 48;

    /// <summary>面板样式：深色底 + 细边框 + 圆角。</summary>
    public static StyleBoxFlat PanelStyle(Color? border = null, Color? fill = null) => new()
    {
        BgColor = fill ?? Panel,
        BorderColor = border ?? PanelBorder,
        BorderWidthLeft = 1,
        BorderWidthTop = 1,
        BorderWidthRight = 1,
        BorderWidthBottom = 1,
        CornerRadiusTopLeft = CornerRadius,
        CornerRadiusTopRight = CornerRadius,
        CornerRadiusBottomLeft = CornerRadius,
        CornerRadiusBottomRight = CornerRadius,
        ContentMarginLeft = PanelPadding,
        ContentMarginRight = PanelPadding,
        ContentMarginTop = PanelPadding,
        ContentMarginBottom = PanelPadding,
    };

    /// <summary>用面板样式包住内容。</summary>
    public static PanelContainer PanelBox(Control content, Color? border = null, Color? fill = null)
    {
        var container = new PanelContainer();
        container.AddThemeStyleboxOverride("panel", PanelStyle(border, fill));
        container.AddChild(content);
        return container;
    }

    public static Label Title(string text) => Styled(new Label { Text = text }, FontTitle, TextPrimary);

    public static Label Heading(string text) => Styled(new Label { Text = text }, FontPanelTitle, TextPrimary);

    public static Label Body(string text, Color? color = null) =>
        Styled(new Label { Text = text }, FontBody, color ?? TextPrimary);

    public static Label Caption(string text, Color? color = null) =>
        Styled(new Label { Text = text }, FontCaption, color ?? TextSecondary);

    /// <summary>生命值颜色：> 50% 主文字，≤ 50% 注意，≤ 25% 危险。</summary>
    public static Color HealthColor(int current, int max)
    {
        if (max <= 0)
        {
            return TextPrimary;
        }
        var ratio = (double)current / max;
        if (ratio <= 0.25)
        {
            return Danger;
        }
        return ratio <= 0.5 ? Warning : TextPrimary;
    }

    /// <summary>按钮底色按强调色深浅生成，保证文字对比度。</summary>
    public static void StyleButton(Button button, Color? accent = null, bool selected = false)
    {
        button.AddThemeFontSizeOverride("font_size", FontBody);
        button.AddThemeColorOverride("font_color", TextPrimary);
        button.AddThemeColorOverride("font_hover_color", Colors.White);
        button.AddThemeColorOverride("font_disabled_color", TextMuted);

        var normal = PanelStyle(selected ? Selection : PanelBorder, selected ? Color.FromHtml("2c2a1e") : Color.FromHtml("262c3a"));
        var hover = PanelStyle(accent ?? Selection, Color.FromHtml("2f3648"));
        var pressed = PanelStyle(accent ?? Selection, Color.FromHtml("38405a"));
        button.AddThemeStyleboxOverride("normal", normal);
        button.AddThemeStyleboxOverride("hover", hover);
        button.AddThemeStyleboxOverride("pressed", pressed);
        button.AddThemeStyleboxOverride("disabled", PanelStyle(PanelBorder, Color.FromHtml("1b1f29")));
        button.AddThemeStyleboxOverride("focus", new StyleBoxEmpty());
    }

    /// <summary>只读信息标签（例如 HP/费用/牌库统计行）。</summary>
    public static Label StatLine(string text) => Styled(new Label { Text = text }, FontBody, TextSecondary);

    private static Label Styled(Label label, int fontSize, Color color)
    {
        label.AddThemeFontSizeOverride("font_size", fontSize);
        label.AddThemeColorOverride("font_color", color);
        return label;
    }
}

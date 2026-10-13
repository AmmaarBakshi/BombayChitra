package bombaychitra.pipeline.util;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Spelling-insensitive forms of Latin-script names for search. Mumbai's names are transliterated in many ways —
 * Chhatrapati / Chatrapati, Bhandup / Bandup, Worli / Vorli, Mattunga / Matunga, Vile Parle / Vileparle — so the
 * search index also holds each name with these differences folded away, and queries are folded the same way.
 *
 * <p>The server implements the same rules ({@code server/internal/store/spelling.go}); both are tested against
 * {@code testdata/search-spelling.tsv}, so keep them in step.
 */
public final class Spelling {

  /** Accented Latin letters seen in Indian names and their plain forms. */
  private static final Map<Character, Character> PLAIN = Map.ofEntries(
    Map.entry('à', 'a'), Map.entry('á', 'a'), Map.entry('â', 'a'), Map.entry('ä', 'a'), Map.entry('ā', 'a'),
    Map.entry('è', 'e'), Map.entry('é', 'e'), Map.entry('ê', 'e'), Map.entry('ë', 'e'), Map.entry('ē', 'e'),
    Map.entry('ì', 'i'), Map.entry('í', 'i'), Map.entry('î', 'i'), Map.entry('ï', 'i'), Map.entry('ī', 'i'),
    Map.entry('ò', 'o'), Map.entry('ó', 'o'), Map.entry('ô', 'o'), Map.entry('ö', 'o'), Map.entry('ō', 'o'),
    Map.entry('ù', 'u'), Map.entry('ú', 'u'), Map.entry('û', 'u'), Map.entry('ü', 'u'), Map.entry('ū', 'u'),
    Map.entry('ñ', 'n'), Map.entry('ṇ', 'n'), Map.entry('ç', 'c'), Map.entry('ṭ', 't'), Map.entry('ḍ', 'd'),
    Map.entry('ṣ', 's'), Map.entry('ś', 's'), Map.entry('ṛ', 'r'), Map.entry('ḥ', 'h'), Map.entry('ṃ', 'm')
  );

  private Spelling() {}

  /** The folded words of a text; words in other scripts (Devanagari…) are left out. */
  public static List<String> words(String text) {
    List<String> out = new ArrayList<>();
    if (text == null) {
      return out;
    }
    StringBuilder word = new StringBuilder();
    boolean latin = true;
    String lower = text.toLowerCase(Locale.ROOT);
    for (int i = 0; i <= lower.length(); i++) {
      char c = i < lower.length() ? lower.charAt(i) : ' ';
      if (Character.isLetterOrDigit(c) || Character.getType(c) == Character.NON_SPACING_MARK
        || Character.getType(c) == Character.COMBINING_SPACING_MARK) {
        char plain = PLAIN.getOrDefault(c, c);
        if (plain >= 'a' && plain <= 'z' || plain >= '0' && plain <= '9') {
          word.append(plain);
        } else {
          latin = false;
        }
        continue;
      }
      if (!word.isEmpty() && latin) {
        out.add(fold(word.toString()));
      }
      word.setLength(0);
      latin = true;
    }
    return out;
  }

  /** Folds one lower-case ASCII word. */
  static String fold(String w) {
    w = w.replace("chh", "ch").replace("ph", "f");
    for (String aspirate : new String[]{"kh", "gh", "th", "dh", "bh", "jh"}) {
      w = w.replace(aspirate, aspirate.substring(0, 1));
    }
    w = w.replace("ee", "i").replace("oo", "u").replace('w', 'v').replace('z', 'j');
    StringBuilder out = new StringBuilder(w.length());
    for (int i = 0; i < w.length(); i++) {
      if (i == 0 || w.charAt(i) != w.charAt(i - 1)) {
        out.append(w.charAt(i));
      }
    }
    return out.toString();
  }

  /**
   * The text to index for a name and its other names (several may be separated by semicolons): every folded word,
   * plus each name's words joined, so "vileparle" finds "Vile Parle". Null when nothing is in Latin script.
   */
  public static String index(String... names) {
    var out = new java.util.LinkedHashSet<String>();
    for (String value : names) {
      if (value == null) {
        continue;
      }
      // OpenStreetMap separates several names in one tag with semicolons (short_name=CSMT;CST;VT).
      for (String name : value.split(";")) {
        List<String> words = words(name);
        out.addAll(words);
        if (words.size() > 1) {
          out.add(String.join("", words));
        }
      }
    }
    return out.isEmpty() ? null : String.join(" ", out);
  }
}

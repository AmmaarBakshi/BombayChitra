package bombaychitra.pipeline.util;

import java.nio.charset.StandardCharsets;
import java.text.Normalizer;
import java.util.LinkedHashSet;
import java.util.Locale;
import java.util.Set;

/** Helpers for Overture Maps records: numeric ids and fuzzy name matching against OpenStreetMap. */
public final class Overture {

  /** Feature-id type digit for Overture records (OpenStreetMap uses 1 node, 2 way, 3 relation). */
  public static final int TYPE_CODE = 4;

  /** Words that do not distinguish one business from another. */
  private static final Set<String> STOPWORDS = Set.of("the", "and", "of", "pvt", "ltd", "private", "limited", "llp",
    "co", "company", "shri", "shree", "sri", "mumbai", "navi", "thane", "branch", "india", "near", "opp", "no", "a",
    "an", "at", "in");

  private Overture() {}

  /**
   * Stable numeric id for an Overture GERS id: a 49-bit FNV-1a hash × 10 + {@value #TYPE_CODE}, so it never collides
   * with OpenStreetMap ids (which end in 1–3) and stays below 2<sup>53</sup>, exact in JavaScript.
   */
  public static long featureId(String gersId) {
    long hash = 0xcbf29ce484222325L;
    for (byte b : gersId.getBytes(StandardCharsets.UTF_8)) {
      hash ^= b & 0xff;
      hash *= 0x100000001b3L;
    }
    return (hash >>> 15) * 10 + TYPE_CODE;
  }

  /** Lower-case, accent-free word set of a name without generic words; empty for null. */
  public static Set<String> nameTokens(String name) {
    Set<String> tokens = new LinkedHashSet<>();
    if (name == null) {
      return tokens;
    }
    String plain = Normalizer.normalize(name, Normalizer.Form.NFKD).replaceAll("\\p{M}", "").toLowerCase(Locale.ROOT);
    for (String token : plain.split("[^\\p{L}\\p{N}]+")) {
      if (!token.isEmpty() && !STOPWORDS.contains(token)) {
        tokens.add(token);
      }
    }
    return tokens;
  }

  /**
   * Words that say what a place is rather than which one it is. Two names that share only these ("HDFC Bank ATM",
   * "ICICI Bank ATM") are different places.
   */
  private static final Set<String> GENERIC = Set.of("bank", "atm", "hospital", "clinic", "nursing", "home", "school",
    "high", "college", "restaurant", "cafe", "hotel", "lodge", "bar", "medical", "medicals", "chemist", "pharmacy",
    "store", "stores", "shop", "general", "mart", "centre", "center", "road", "station", "temple", "mandir",
    "church", "mosque", "masjid", "society", "building", "tower", "towers", "apartment", "apartments", "garden",
    "park", "gym", "salon", "dental", "hall", "office", "petrol", "pump", "police", "post");

  /**
   * True when two names share most of their words, or one name's words are all in the other — and, when both names
   * have distinctive words, they share at least one of those.
   */
  public static boolean similar(Set<String> a, Set<String> b) {
    if (a.isEmpty() || b.isEmpty()) {
      return false;
    }
    int common = 0;
    boolean distinctiveA = false;
    boolean distinctiveB = false;
    boolean distinctiveCommon = false;
    for (String token : a) {
      boolean generic = GENERIC.contains(token);
      distinctiveA |= !generic;
      if (b.contains(token)) {
        common++;
        distinctiveCommon |= !generic;
      }
    }
    for (String token : b) {
      distinctiveB |= !GENERIC.contains(token);
    }
    if (common == 0 || distinctiveA && distinctiveB && !distinctiveCommon) {
      return false;
    }
    int union = a.size() + b.size() - common;
    return common == Math.min(a.size(), b.size()) || common * 2 >= union;
  }
}

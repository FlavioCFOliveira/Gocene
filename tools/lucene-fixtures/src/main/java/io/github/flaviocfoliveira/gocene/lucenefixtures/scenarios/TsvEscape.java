package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

/**
 * Sprint 114 T14 (rmp 4622) TSV escape helper shared by the two
 * highlight scenarios. Encodes the four characters that would break a
 * tab-separated row ({@code \, \t, \n, \r}) and round-trips them on read.
 */
final class TsvEscape {
    private TsvEscape() {}

    static String escape(String s) {
        return s.replace("\\", "\\\\").replace("\t", "\\t")
                .replace("\n", "\\n").replace("\r", "\\r");
    }

    static String unescape(String s) {
        StringBuilder sb = new StringBuilder(s.length());
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            if (c == '\\' && i + 1 < s.length()) {
                char n = s.charAt(++i);
                switch (n) {
                    case '\\': sb.append('\\'); break;
                    case 't': sb.append('\t'); break;
                    case 'n': sb.append('\n'); break;
                    case 'r': sb.append('\r'); break;
                    default: sb.append('\\').append(n);
                }
            } else {
                sb.append(c);
            }
        }
        return sb.toString();
    }
}

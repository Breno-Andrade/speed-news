import { SkeletonBlock, SkeletonGroup } from "../../shared/ui/Skeleton";

const placeholderRows = [
  { titleWidth: "92%", secondLineWidth: "64%", hasThumbnail: true },
  { titleWidth: "86%", secondLineWidth: "48%", hasThumbnail: false },
  { titleWidth: "95%", secondLineWidth: "70%", hasThumbnail: true },
  { titleWidth: "80%", secondLineWidth: "55%", hasThumbnail: true },
  { titleWidth: "90%", secondLineWidth: "40%", hasThumbnail: false },
];

/** Same geometry as a news item, so nothing jumps when the real list arrives. Shown at once (it
 *  continues the startup skeleton), so it does not fade in. */
export function NewsListSkeleton({ label }: { label: string }) {
  return (
    <SkeletonGroup label={label} className="skeleton-group--instant">
      <ul className="news-list">
        {placeholderRows.map((placeholderRow, rowIndex) => (
          <li key={rowIndex} className="news-item news-item--skeleton">
            <span className="news-item__dot" />
            <span className="news-item__text">
              <SkeletonBlock width={placeholderRow.titleWidth} height={13} />
              <SkeletonBlock width={placeholderRow.secondLineWidth} height={13} />
              <SkeletonBlock width={110} height={9} />
            </span>
            {placeholderRow.hasThumbnail && <SkeletonBlock width={64} height={48} radius={6} />}
          </li>
        ))}
      </ul>
    </SkeletonGroup>
  );
}
